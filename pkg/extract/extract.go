package extract

import (
	"errors"
	"fmt"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/operator-framework/api/pkg/manifests"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Manifests extracts all Kubernetes manifests from an OLM bundle for the given namespace.
// Returns objects in the proper order for kubectl apply.
func Manifests(bundle *manifests.Bundle, namespace string) ([]runtime.Object, error) {
	if bundle.CSV == nil {
		return nil, errors.New("bundle does not contain a ClusterServiceVersion")
	}

	objects := make([]runtime.Object, 0)

	// 1. Namespace (if not "default").
	if namespace != "default" {
		objects = append(objects, kube.CreateNamespace(namespace))
	}

	// 2. CRDs (with conversion webhook config if applicable).
	crds := CRDs(bundle, bundle.CSV, namespace)
	objects = append(objects, crds...)

	// 3-6. RBAC and Deployments from CSV InstallStrategy.
	installObjects, err := InstallStrategy(bundle.CSV, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to convert install strategy: %w", err)
	}

	objects = append(objects, installObjects...)

	// 7. Webhook Services (must exist before webhooks reference them).
	webhookServices := WebhookServices(bundle.CSV, namespace)
	objects = append(objects, webhookServices...)

	// 8. ValidatingWebhookConfigurations and MutatingWebhookConfigurations.
	webhooks := Webhooks(bundle.CSV, namespace)
	objects = append(objects, webhooks...)

	// 9. Other resources from bundle.
	otherObjects := OtherResources(bundle, namespace)
	objects = append(objects, otherObjects...)

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
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
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
			APIVersion: apiextensionsv1beta1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
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

	// Extract resources from OperatorPermissions, maintaining proper order.
	// First pass: collect all ServiceAccounts.
	for _, perm := range permissions {
		if perm.ServiceAccount != nil {
			sa := perm.ServiceAccount.DeepCopy()
			sa.TypeMeta = metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "ServiceAccount",
			}
			// Clear OLM-specific owner references for standalone installation.
			sa.OwnerReferences = nil

			objects = append(objects, sa)
		}
	}

	// Second pass: collect all Roles and RoleBindings.
	for _, perm := range permissions {
		for _, role := range perm.Roles {
			roleCopy := role.DeepCopy()
			roleCopy.TypeMeta = metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "Role",
			}
			roleCopy.OwnerReferences = nil
			roleCopy.Labels = nil

			objects = append(objects, roleCopy)
		}

		for _, rb := range perm.RoleBindings {
			rbCopy := rb.DeepCopy()
			rbCopy.TypeMeta = metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "RoleBinding",
			}
			rbCopy.OwnerReferences = nil
			rbCopy.Labels = nil

			objects = append(objects, rbCopy)
		}
	}

	// Third pass: collect all ClusterRoles and ClusterRoleBindings.
	for _, perm := range permissions {
		for _, cr := range perm.ClusterRoles {
			crCopy := cr.DeepCopy()
			crCopy.TypeMeta = metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRole",
			}
			crCopy.Labels = nil

			objects = append(objects, crCopy)
		}

		for _, crb := range perm.ClusterRoleBindings {
			crbCopy := crb.DeepCopy()
			crbCopy.TypeMeta = metav1.TypeMeta{
				APIVersion: rbacv1.SchemeGroupVersion.String(),
				Kind:       "ClusterRoleBinding",
			}
			crbCopy.Labels = nil

			objects = append(objects, crbCopy)
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

// OtherResources extracts non-CRD, non-CSV resources from the bundle.
func OtherResources(bundle *manifests.Bundle, namespace string) []runtime.Object {
	objects := make([]runtime.Object, 0, len(bundle.Objects))

	for _, obj := range bundle.Objects {
		gvk := obj.GetObjectKind().GroupVersionKind()

		// Skip OLM-specific resources and CRDs (already handled).
		switch gvk.Kind {
		case "ClusterServiceVersion", "CustomResourceDefinition":
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
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "ValidatingWebhookConfiguration",
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
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "MutatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: desc.GenerateName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{webhook},
	}
}
