package extract

import (
	"errors"
	"fmt"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/operator-framework/api/pkg/manifests"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/registry/resolver"

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

	// 2. CRDs.
	crds := CRDs(bundle)
	objects = append(objects, crds...)

	// 3-6. RBAC and Deployments from CSV InstallStrategy.
	installObjects, err := InstallStrategy(bundle.CSV, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to convert install strategy: %w", err)
	}

	objects = append(objects, installObjects...)

	// 7. Other resources from bundle.
	otherObjects := OtherResources(bundle, namespace)
	objects = append(objects, otherObjects...)

	return objects, nil
}

// CRDs extracts CustomResourceDefinitions from the bundle.
func CRDs(bundle *manifests.Bundle) []runtime.Object {
	objects := make([]runtime.Object, 0, len(bundle.V1CRDs)+len(bundle.V1beta1CRDs))

	// v1 CRDs.
	for _, crd := range bundle.V1CRDs {
		crdCopy := crd.DeepCopy()
		crdCopy.TypeMeta = metav1.TypeMeta{
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
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
		if kube.IsNamespaced(gvk.Kind) {
			kube.SetNamespace(obj, namespace)
		}

		objects = append(objects, obj)
	}

	return objects
}
