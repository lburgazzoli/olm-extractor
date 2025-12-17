package kube

import (
	"errors"
	"fmt"
	"sort"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
)

// Convert converts a runtime.Object to the specified concrete type T and returns a deep copy.
// It handles both direct type assertions and unstructured objects by attempting conversion.
// This is useful when you need to work with typed objects while ensuring immutability.
func Convert[T runtime.Object](obj runtime.Object) (T, error) {
	var zero T

	// Try direct type assertion first
	if typed, ok := obj.(T); ok {
		copied := typed.DeepCopyObject()
		result, ok := copied.(T)
		if !ok {
			return zero, errors.New("deep copy returned unexpected type")
		}

		return result, nil
	}

	// If object is unstructured, try to convert it to the concrete type
	if u, ok := obj.(*unstructured.Unstructured); ok {
		var target T
		if err := FromUnstructured(u, &target); err != nil {
			return zero, fmt.Errorf("failed to convert unstructured to %T: %w", zero, err)
		}

		return target, nil
	}

	return zero, fmt.Errorf("object is not of type %T and not unstructured", zero)
}

// ToUnstructured converts a typed Kubernetes object to an Unstructured object.
func ToUnstructured(obj any) (*unstructured.Unstructured, error) {
	// Fast path: already unstructured
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u, nil
	}

	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}

	return &unstructured.Unstructured{Object: unstructuredMap}, nil
}

// CleanUnstructured removes nil values and empty maps/slices from an Unstructured object.
// Returns a new Unstructured object with cleaned data.
func CleanUnstructured(obj *unstructured.Unstructured) *unstructured.Unstructured {
	cleaned := cleanMap(obj.Object)

	return &unstructured.Unstructured{Object: cleaned}
}

// cleanMap recursively removes nil values and empty maps/slices from a map.
func cleanMap(obj map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range obj {
		if cleaned := cleanValue(value); cleaned != nil {
			result[key] = cleaned
		}
	}

	return result
}

// cleanValue recursively cleans a value by removing nil and empty collections.
func cleanValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case map[string]any:
		cleaned := cleanMap(v)
		if len(cleaned) == 0 {
			return nil
		}

		return cleaned
	case []any:
		if len(v) == 0 {
			return nil
		}

		cleaned := make([]any, 0, len(v))

		for _, item := range v {
			// Preserve empty strings in arrays (e.g. apiGroups: [""] = core API in Kubernetes)
			if str, ok := item.(string); ok && str == "" {
				cleaned = append(cleaned, "")

				continue
			}

			if c := cleanValue(item); c != nil {
				cleaned = append(cleaned, c)
			}
		}

		if len(cleaned) == 0 {
			return nil
		}

		return cleaned
	case string:
		if v == "" {
			return nil
		}

		return v
	default:
		return value
	}
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
			APIVersion: gvks.Namespace.GroupVersion().String(),
			Kind:       gvks.Namespace.Kind,
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
			APIVersion: gvks.Deployment.GroupVersion().String(),
			Kind:       gvks.Deployment.Kind,
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
			APIVersion: gvks.Service.GroupVersion().String(),
			Kind:       gvks.Service.Kind,
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

// SetNamespace sets the namespace on a runtime.Object.
func SetNamespace(obj runtime.Object, namespace string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to get object accessor: %w", err)
	}

	accessor.SetNamespace(namespace)

	return nil
}

// ValidateNamespace validates a Kubernetes namespace name according to DNS-1123 label standards.
func ValidateNamespace(ns string) error {
	if ns == "" {
		return errors.New("namespace cannot be empty")
	}

	if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
		return fmt.Errorf("invalid namespace name: %s", errs[0])
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

// SortForApply sorts unstructured objects by their resource type priority for proper kubectl apply order.
// Ordering: Namespace → CRD → ServiceAccount → Role → RoleBinding → ClusterRole →
// ClusterRoleBinding → Deployment → Service → Issuer → Certificate → Webhook → Other.
func SortForApply(objects []*unstructured.Unstructured) {
	sort.Slice(objects, func(i int, j int) bool {
		return getUnstructuredPriority(objects[i]) < getUnstructuredPriority(objects[j])
	})
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
	priorityIssuer // cert-manager Issuers must come before Certificates that reference them
	priorityCertificate
	priorityWebhook
	priorityOther
)

// getUnstructuredPriority returns the application priority for an unstructured object.
func getUnstructuredPriority(obj *unstructured.Unstructured) int {
	kind := obj.GetKind()

	switch kind {
	case "Namespace":
		return priorityNamespace
	case "CustomResourceDefinition":
		return priorityCRD
	case "ServiceAccount":
		return priorityServiceAccount
	case "Role":
		return priorityRole
	case "RoleBinding":
		return priorityRoleBinding
	case "ClusterRole":
		return priorityClusterRole
	case "ClusterRoleBinding":
		return priorityClusterRoleBinding
	case "Deployment":
		return priorityDeployment
	case "Service":
		return priorityService
	case "Issuer", "ClusterIssuer":
		return priorityIssuer
	case "Certificate":
		return priorityCertificate
	case "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration":
		return priorityWebhook
	default:
		return priorityOther
	}
}
