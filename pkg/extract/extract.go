package extract

import (
	"errors"
	"fmt"
	"sort"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
	"github.com/operator-framework/api/pkg/manifests"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Manifests extracts all Kubernetes manifests from an OLM bundle for the given namespace.
// Returns objects sorted by type priority for proper kubectl apply order.
func Manifests(bundle *manifests.Bundle, namespace string) ([]runtime.Object, error) {
	if bundle.CSV == nil {
		return nil, errors.New("bundle does not contain a ClusterServiceVersion")
	}

	objects := make([]runtime.Object, 0)

	// Namespace (if not "default").
	if namespace != "default" {
		objects = append(objects, kube.CreateNamespace(namespace))
	}

	// CRDs (with conversion webhook config if applicable).
	crds := CRDs(bundle, bundle.CSV, namespace)
	objects = append(objects, crds...)

	// RBAC and Deployments from CSV InstallStrategy.
	installObjects, err := InstallStrategy(bundle.CSV, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to convert install strategy: %w", err)
	}

	objects = append(objects, installObjects...)

	// Webhook Services.
	webhookServices := WebhookServices(bundle.CSV, namespace)
	objects = append(objects, webhookServices...)

	// ValidatingWebhookConfigurations and MutatingWebhookConfigurations.
	webhooks := Webhooks(bundle.CSV, namespace)
	objects = append(objects, webhooks...)

	// Other resources from bundle.
	otherObjects := OtherResources(bundle, namespace)
	objects = append(objects, otherObjects...)

	// Sort resources by priority for proper kubectl apply order.
	objects = sortKubernetesResources(objects)

	return objects, nil
}

// CRDs extracts CustomResourceDefinitions from the bundle.
// If the CSV defines ConversionWebhooks, the CRDs are patched with conversion configuration.
func CRDs(bundle *manifests.Bundle, csv *v1alpha1.ClusterServiceVersion, namespace string) []runtime.Object {
	objects := make([]runtime.Object, 0, len(bundle.V1CRDs)+len(bundle.V1beta1CRDs))

	// Build a map of CRDs that need conversion webhooks.
	conversionWebhooks := buildConversionWebhookMap(csv, namespace)

	// v1 CRDs.
	for _, crd := range bundle.V1CRDs {
		crdCopy := crd.DeepCopy()
		crdCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.CustomResourceDefinition.GroupVersion().String(),
			Kind:       gvks.CustomResourceDefinition.Kind,
		}

		// Apply conversion webhook config if defined.
		if convConfig, ok := conversionWebhooks[crdCopy.Name]; ok {
			crdCopy.Spec.Conversion = convConfig
		}

		objects = append(objects, crdCopy)
	}

	// v1beta1 CRDs.
	for _, crd := range bundle.V1beta1CRDs {
		crdCopy := crd.DeepCopy()
		crdCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.CustomResourceDefinitionV1Beta1.GroupVersion().String(),
			Kind:       gvks.CustomResourceDefinitionV1Beta1.Kind,
		}

		objects = append(objects, crdCopy)
	}

	return objects
}

// buildConversionWebhookMap builds a map from CRD name to conversion config.
func buildConversionWebhookMap(
	csv *v1alpha1.ClusterServiceVersion,
	namespace string,
) map[string]*apiextensionsv1.CustomResourceConversion {
	conversionWebhooks := make(map[string]*apiextensionsv1.CustomResourceConversion)

	if csv == nil {
		return conversionWebhooks
	}

	for _, desc := range csv.Spec.WebhookDefinitions {
		if desc.Type != v1alpha1.ConversionWebhook {
			continue
		}

		// Build the conversion config for each CRD this webhook applies to.
		for _, crdName := range desc.ConversionCRDs {
			conversionWebhooks[crdName] = createConversionConfig(desc, namespace)
		}
	}

	return conversionWebhooks
}

// createConversionConfig creates a CustomResourceConversion config from a WebhookDescription.
func createConversionConfig(
	desc v1alpha1.WebhookDescription,
	namespace string,
) *apiextensionsv1.CustomResourceConversion {
	port := desc.ContainerPort
	if port == 0 {
		port = kube.DefaultWebhookServicePort
	}

	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: namespace,
					Name:      desc.DeploymentName + "-webhook-service",
					Path:      desc.WebhookPath,
					Port:      &port,
				},
				// CA bundle left empty - users must inject certificates.
				CABundle: nil,
			},
			ConversionReviewVersions: desc.AdmissionReviewVersions,
		},
	}
}

// InstallStrategy converts a CSV install strategy to Kubernetes resources.
// Returns ServiceAccounts, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings, and Deployments.
func InstallStrategy(csv *v1alpha1.ClusterServiceVersion, namespace string) ([]runtime.Object, error) {
	strategy := csv.Spec.InstallStrategy
	if strategy.StrategyName != v1alpha1.InstallStrategyNameDeployment && strategy.StrategyName != "" {
		return nil, fmt.Errorf("unsupported install strategy: %s", strategy.StrategyName)
	}

	// Set CSV namespace for OLM's RBACForClusterServiceVersion to use.
	csv.SetNamespace(namespace)

	// Use OLM's RBACForClusterServiceVersion to generate RBAC resources.
	permissions, err := resolver.RBACForClusterServiceVersion(csv)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RBAC from CSV: %w", err)
	}

	objects := make([]runtime.Object, 0)

	// Extract resources from OperatorPermissions in a single pass.
	// Helper functions maintain proper ordering per permission.
	for _, perms := range permissions {
		// ServiceAccount
		if sa := processServiceAccount(perms); sa != nil {
			objects = append(objects, sa)
		}

		// Roles
		for _, role := range processRoles(perms) {
			objects = append(objects, role)
		}

		// RoleBindings
		for _, rb := range processRoleBindings(perms) {
			objects = append(objects, rb)
		}

		// ClusterRoles
		for _, cr := range processClusterRoles(perms) {
			objects = append(objects, cr)
		}

		// ClusterRoleBindings
		for _, crb := range processClusterRoleBindings(perms) {
			objects = append(objects, crb)
		}
	}

	// Add Deployments from the install strategy.
	spec := strategy.StrategySpec
	for _, depSpec := range spec.DeploymentSpecs {
		deployment := kube.CreateDeployment(depSpec, namespace)
		objects = append(objects, deployment)
	}

	return objects, nil
}

// processServiceAccount processes a ServiceAccount from OperatorPermissions.
func processServiceAccount(perm *resolver.OperatorPermissions) *corev1.ServiceAccount {
	if perm.ServiceAccount == nil {
		return nil
	}

	sa := perm.ServiceAccount.DeepCopy()
	sa.TypeMeta = metav1.TypeMeta{
		APIVersion: gvks.ServiceAccount.GroupVersion().String(),
		Kind:       gvks.ServiceAccount.Kind,
	}
	// Clear OLM-specific owner references for standalone installation.
	sa.OwnerReferences = nil

	return sa
}

// processRoles processes all Roles from OperatorPermissions.
func processRoles(perm *resolver.OperatorPermissions) []*rbacv1.Role {
	if len(perm.Roles) == 0 {
		return nil
	}

	roles := make([]*rbacv1.Role, 0, len(perm.Roles))
	for _, role := range perm.Roles {
		roleCopy := role.DeepCopy()
		roleCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.Role.GroupVersion().String(),
			Kind:       gvks.Role.Kind,
		}
		roleCopy.OwnerReferences = nil
		roleCopy.Labels = nil
		roleCopy.Rules = normalizeRBACRules(roleCopy.Rules)

		roles = append(roles, roleCopy)
	}

	return roles
}

// processRoleBindings processes all RoleBindings from OperatorPermissions.
func processRoleBindings(perm *resolver.OperatorPermissions) []*rbacv1.RoleBinding {
	if len(perm.RoleBindings) == 0 {
		return nil
	}

	bindings := make([]*rbacv1.RoleBinding, 0, len(perm.RoleBindings))
	for _, rb := range perm.RoleBindings {
		rbCopy := rb.DeepCopy()
		rbCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.RoleBinding.GroupVersion().String(),
			Kind:       gvks.RoleBinding.Kind,
		}
		rbCopy.OwnerReferences = nil
		rbCopy.Labels = nil

		bindings = append(bindings, rbCopy)
	}

	return bindings
}

// processClusterRoles processes all ClusterRoles from OperatorPermissions.
func processClusterRoles(perm *resolver.OperatorPermissions) []*rbacv1.ClusterRole {
	if len(perm.ClusterRoles) == 0 {
		return nil
	}

	clusterRoles := make([]*rbacv1.ClusterRole, 0, len(perm.ClusterRoles))
	for _, cr := range perm.ClusterRoles {
		crCopy := cr.DeepCopy()
		crCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.ClusterRole.GroupVersion().String(),
			Kind:       gvks.ClusterRole.Kind,
		}
		crCopy.Labels = nil
		crCopy.Rules = normalizeRBACRules(crCopy.Rules)

		clusterRoles = append(clusterRoles, crCopy)
	}

	return clusterRoles
}

// processClusterRoleBindings processes all ClusterRoleBindings from OperatorPermissions.
func processClusterRoleBindings(perm *resolver.OperatorPermissions) []*rbacv1.ClusterRoleBinding {
	if len(perm.ClusterRoleBindings) == 0 {
		return nil
	}

	bindings := make([]*rbacv1.ClusterRoleBinding, 0, len(perm.ClusterRoleBindings))
	for _, crb := range perm.ClusterRoleBindings {
		crbCopy := crb.DeepCopy()
		crbCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: gvks.ClusterRoleBinding.GroupVersion().String(),
			Kind:       gvks.ClusterRoleBinding.Kind,
		}
		crbCopy.Labels = nil

		bindings = append(bindings, crbCopy)
	}

	return bindings
}

// normalizeRBACRules ensures all RBAC rules have apiGroups field.
// Kubernetes requires apiGroups even for core resources (use empty string "").
func normalizeRBACRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	normalized := make([]rbacv1.PolicyRule, len(rules))
	for i, rule := range rules {
		normalized[i] = rule
		// If rule has resources but no apiGroups, default to core API group
		if len(rule.Resources) > 0 && len(rule.APIGroups) == 0 {
			normalized[i].APIGroups = []string{""}
		}
	}

	return normalized
}

// OtherResources extracts non-CRD, non-CSV resources from the bundle.
func OtherResources(bundle *manifests.Bundle, namespace string) []runtime.Object {
	objects := make([]runtime.Object, 0, len(bundle.Objects))

	for _, obj := range bundle.Objects {
		gvk := obj.GetObjectKind().GroupVersionKind()

		// Skip OLM-specific resources and CRDs (already handled).
		switch gvk {
		case gvks.ClusterServiceVersion, gvks.CustomResourceDefinition, gvks.CustomResourceDefinitionV1Beta1:
			continue
		}

		// Set namespace for namespaced resources.
		if kube.IsNamespaced(gvk) {
			kube.SetNamespace(obj, namespace)
		}

		objects = append(objects, obj)
	}

	return objects
}

// Webhooks extracts ValidatingWebhookConfiguration and MutatingWebhookConfiguration
// from the CSV's WebhookDefinitions. CA bundles are left empty - users must inject
// certificates (e.g., via cert-manager or manual configuration).
func Webhooks(csv *v1alpha1.ClusterServiceVersion, namespace string) []runtime.Object {
	webhookDefs := csv.Spec.WebhookDefinitions
	if len(webhookDefs) == 0 {
		return nil
	}

	objects := make([]runtime.Object, 0, len(webhookDefs))

	for _, desc := range webhookDefs {
		switch desc.Type {
		case v1alpha1.ValidatingAdmissionWebhook:
			vwc := createValidatingWebhookConfiguration(desc, namespace)
			objects = append(objects, vwc)

		case v1alpha1.MutatingAdmissionWebhook:
			mwc := createMutatingWebhookConfiguration(desc, namespace)
			objects = append(objects, mwc)

		case v1alpha1.ConversionWebhook:
			// ConversionWebhooks are handled separately by patching CRDs.
			continue
		}
	}

	return objects
}

// WebhookServices creates Services for webhook deployments.
// Each unique deployment referenced by webhooks gets a Service.
func WebhookServices(csv *v1alpha1.ClusterServiceVersion, namespace string) []runtime.Object {
	webhookDefs := csv.Spec.WebhookDefinitions
	if len(webhookDefs) == 0 {
		return nil
	}

	// Track unique deployments to avoid creating duplicate Services.
	seen := make(map[string]bool)
	objects := make([]runtime.Object, 0)

	for _, desc := range webhookDefs {
		if desc.DeploymentName == "" {
			continue
		}

		if seen[desc.DeploymentName] {
			continue
		}
		seen[desc.DeploymentName] = true

		svc := kube.CreateWebhookService(desc.DeploymentName, namespace, desc.ContainerPort, desc.TargetPort)
		objects = append(objects, svc)
	}

	return objects
}

// createValidatingWebhookConfiguration creates a ValidatingWebhookConfiguration from a WebhookDescription.
func createValidatingWebhookConfiguration(
	desc v1alpha1.WebhookDescription,
	namespace string,
) *admissionregistrationv1.ValidatingWebhookConfiguration {
	// Get the webhook from the description helper method.
	// Pass nil for namespaceSelector and empty caBundle - users must inject certificates.
	webhook := desc.GetValidatingWebhook(namespace, nil, nil)

	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gvks.ValidatingWebhookConfiguration.GroupVersion().String(),
			Kind:       gvks.ValidatingWebhookConfiguration.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desc.GenerateName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{webhook},
	}
}

// createMutatingWebhookConfiguration creates a MutatingWebhookConfiguration from a WebhookDescription.
func createMutatingWebhookConfiguration(
	desc v1alpha1.WebhookDescription,
	namespace string,
) *admissionregistrationv1.MutatingWebhookConfiguration {
	// Get the webhook from the description helper method.
	// Pass nil for namespaceSelector and empty caBundle - users must inject certificates.
	webhook := desc.GetMutatingWebhook(namespace, nil, nil)

	return &admissionregistrationv1.MutatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gvks.MutatingWebhookConfiguration.GroupVersion().String(),
			Kind:       gvks.MutatingWebhookConfiguration.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desc.GenerateName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{webhook},
	}
}

// Resource priority constants for kubectl apply ordering.
const (
	priorityNamespace = 1 + iota
	priorityCRD
	priorityServiceAccount
	priorityRole
	priorityRoleBinding
	priorityClusterRole
	priorityClusterRoleBinding
	priorityDeployment
	priorityService
	priorityCertificate // cert-manager Certificates must come before webhooks that use them
	priorityWebhook
	priorityOther
)

// sortKubernetesResources sorts resources by their type priority for proper kubectl apply order.
// Ordering: Namespace → CRD → ServiceAccount → Role → RoleBinding → ClusterRole →
// ClusterRoleBinding → Deployment → Service → Certificate → Webhook → Other.
func sortKubernetesResources(objects []runtime.Object) []runtime.Object {
	// Create a copy to avoid modifying the original slice
	sorted := make([]runtime.Object, len(objects))
	copy(sorted, objects)

	// Sort by priority (lower numbers first)
	sort.Slice(sorted, func(i int, j int) bool {
		return getResourcePriority(sorted[i]) < getResourcePriority(sorted[j])
	})

	return sorted
}

// getResourcePriority returns the priority order for a resource type.
// Lower numbers are applied first.
func getResourcePriority(obj runtime.Object) int {
	gvk := obj.GetObjectKind().GroupVersionKind()

	switch gvk {
	case gvks.Namespace:
		return priorityNamespace
	case gvks.CustomResourceDefinition, gvks.CustomResourceDefinitionV1Beta1:
		return priorityCRD
	case gvks.ServiceAccount:
		return priorityServiceAccount
	case gvks.Role:
		return priorityRole
	case gvks.RoleBinding:
		return priorityRoleBinding
	case gvks.ClusterRole:
		return priorityClusterRole
	case gvks.ClusterRoleBinding:
		return priorityClusterRoleBinding
	case gvks.Deployment:
		return priorityDeployment
	case gvks.Service:
		return priorityService
	case gvks.Certificate:
		return priorityCertificate
	case gvks.ValidatingWebhookConfiguration, gvks.MutatingWebhookConfiguration:
		return priorityWebhook
	default:
		return priorityOther
	}
}
