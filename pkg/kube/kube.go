package kube

import (
	"errors"
	"fmt"

	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ToUnstructured converts a typed Kubernetes object to an Unstructured object.
func ToUnstructured(obj any) (*unstructured.Unstructured, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	return &unstructured.Unstructured{Object: unstructuredMap}, nil
}

// FromUnstructured converts an Unstructured object to a typed Kubernetes object.
func FromUnstructured(u *unstructured.Unstructured, obj any) error {
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
	if err != nil {
		return fmt.Errorf("failed to convert from unstructured: %w", err)
	}

	return nil
}

// Is returns true if the object matches the specified GroupVersionKind and name.
func Is(obj *unstructured.Unstructured, gvk schema.GroupVersionKind, name string) bool {
	return obj.GroupVersionKind() == gvk && obj.GetName() == name
}

// IsKind returns true if the object's kind matches the specified GroupVersionKind.
// It compares only the Group and Kind, ignoring the Version.
func IsKind(obj *unstructured.Unstructured, gvk schema.GroupVersionKind) bool {
	objGVK := obj.GroupVersionKind()

	return objGVK.Group == gvk.Group && objGVK.Kind == gvk.Kind
}

// IsWebhookConfiguration returns true if the object is either a ValidatingWebhookConfiguration
// or MutatingWebhookConfiguration.
func IsWebhookConfiguration(obj *unstructured.Unstructured) bool {
	return IsKind(obj, gvks.ValidatingWebhookConfiguration) || IsKind(obj, gvks.MutatingWebhookConfiguration)
}

// HasAnnotation returns true if the object has the specified annotation.
// Works with any Kubernetes object (typed or unstructured).
func HasAnnotation(obj metav1.Object, annotation string) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, exists := annotations[annotation]

	return exists
}

// SetAnnotation sets an annotation on the object with the given value.
// Works with any Kubernetes object (typed or unstructured).
func SetAnnotation(obj metav1.Object, annotation string, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotation] = value
	obj.SetAnnotations(annotations)
}

// Find returns all objects matching the predicate function.
func Find(objects []*unstructured.Unstructured, predicate func(*unstructured.Unstructured) bool) []*unstructured.Unstructured {
	result := make([]*unstructured.Unstructured, 0)

	for _, obj := range objects {
		if predicate(obj) {
			result = append(result, obj)
		}
	}

	return result
}

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

const (
	// DefaultWebhookServicePort is the default port for webhook services.
	DefaultWebhookServicePort = 443
)

// CreateWebhookService creates a Service for a webhook deployment.
// This is a simplified helper for basic webhook service creation.
// For more advanced scenarios with deployment info extraction, see service.go functions.
func CreateWebhookService(
	deploymentName string,
	namespace string,
	port int32,
	targetPort *intstr.IntOrString,
) *corev1.Service {
	servicePort := port
	if servicePort == 0 {
		servicePort = DefaultWebhookServicePort
	}

	tp := intstr.FromInt32(servicePort)
	if targetPort != nil {
		tp = *targetPort
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-webhook-service",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"name": deploymentName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       servicePort,
					TargetPort: tp,
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// IsNamespaced returns true if the given GroupVersionKind is namespace-scoped.
func IsNamespaced(gvk schema.GroupVersionKind) bool {
	return !gvks.ClusterScoped[gvk]
}

// SetNamespace sets the namespace on a runtime.Object if it implements metav1.Object.
func SetNamespace(obj runtime.Object, namespace string) {
	if accessor, ok := obj.(metav1.Object); ok {
		accessor.SetNamespace(namespace)
	}
}

// ValidateNamespace validates a Kubernetes namespace name according to DNS-1123 label standards.
func ValidateNamespace(ns string) error {
	if ns == "" {
		return errors.New("namespace cannot be empty")
	}

	if len(ns) > 63 {
		return errors.New("namespace name too long (max 63 characters)")
	}

	for i, c := range ns {
		isLowerAlpha := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isDash := c == '-'

		if !isLowerAlpha && !isDigit && !isDash {
			return errors.New("invalid namespace name: must consist of lowercase alphanumeric characters or '-'")
		}

		if i == 0 && (isDash || isDigit) {
			return errors.New("invalid namespace name: must start with a lowercase letter")
		}

		if i == len(ns)-1 && isDash {
			return errors.New("invalid namespace name: must end with an alphanumeric character")
		}
	}

	return nil
}

// ConvertToUnstructured converts a slice of runtime.Objects to Unstructured objects.
func ConvertToUnstructured(objects []runtime.Object) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, 0, len(objects))

	for _, obj := range objects {
		objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
		}

		u := &unstructured.Unstructured{Object: objMap}
		result = append(result, u)
	}

	return result, nil
}
