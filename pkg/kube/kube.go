package kube

import (
	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateNamespace creates a Namespace object with the given name.
func CreateNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateDeployment creates a Deployment from a CSV StrategyDeploymentSpec.
func CreateDeployment(depSpec v1alpha1.StrategyDeploymentSpec, namespace string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      depSpec.Name,
			Namespace: namespace,
			Labels:    depSpec.Label,
		},
		Spec: depSpec.Spec,
	}

	// Ensure namespace is set in the spec template.
	deployment.Spec.Template.Namespace = namespace

	return deployment
}

// IsNamespaced returns true if the given Kind is namespace-scoped.
func IsNamespaced(kind string) bool {
	return !clusterScopedKinds[kind]
}

// SetNamespace sets the namespace on a runtime.Object if it implements metav1.Object.
func SetNamespace(obj runtime.Object, namespace string) {
	if accessor, ok := obj.(metav1.Object); ok {
		accessor.SetNamespace(namespace)
	}
}

// clusterScopedKinds contains Kubernetes resource kinds that are cluster-scoped.
var clusterScopedKinds = map[string]bool{ //nolint:gochecknoglobals
	"Namespace":                      true,
	"CustomResourceDefinition":       true,
	"ClusterRole":                    true,
	"ClusterRoleBinding":             true,
	"PersistentVolume":               true,
	"StorageClass":                   true,
	"PriorityClass":                  true,
	"VolumeSnapshotClass":            true,
	"IngressClass":                   true,
	"RuntimeClass":                   true,
	"PodSecurityPolicy":              true,
	"ClusterIssuer":                  true,
	"ValidatingWebhookConfiguration": true,
	"MutatingWebhookConfiguration":   true,
}
