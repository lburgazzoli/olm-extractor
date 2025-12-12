package cainjection

import (
	"fmt"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CAProvider defines the interface for CA certificate providers.
type CAProvider interface {
	// Name returns the provider name
	Name() string
	// ConfigureWebhook configures a webhook with CA injection annotations/resources
	ConfigureWebhook(webhook *unstructured.Unstructured, serviceName string, namespace string) ([]*unstructured.Unstructured, error)
}

// Configure analyzes filtered resources and configures CA injection for webhooks.
// It uses the provided CAProvider to configure webhooks and ensures services exist.
func Configure(objects []*unstructured.Unstructured, namespace string, provider CAProvider) ([]*unstructured.Unstructured, error) {
	webhooks := findWebhooks(objects)
	if len(webhooks) == 0 {
		return objects, nil
	}

	result := make([]*unstructured.Unstructured, 0, len(objects))
	processedServices := make(map[string]bool)
	addedCertificates := make(map[string]bool)

	for _, obj := range objects {
		kind := obj.GetKind()

		if kind == "ValidatingWebhookConfiguration" || kind == "MutatingWebhookConfiguration" {
			info := extractWebhookInfo(obj, kind)
			if info == nil {
				result = append(result, obj)
				continue
			}

			// Use provider to configure webhook
			providerResources, err := provider.ConfigureWebhook(obj, info.serviceName, namespace)
			if err != nil {
				return nil, fmt.Errorf("failed to configure webhook %s with %s: %w", obj.GetName(), provider.Name(), err)
			}

			// Add provider-specific resources (like Certificates) before webhook
			for _, res := range providerResources {
				resKind := res.GetKind()
				if resKind == "Certificate" || resKind == "ConfigMap" {
					resName := res.GetName()
					if !addedCertificates[resName] {
						result = append(result, res)
						addedCertificates[resName] = true
					}
				} else if resKind == "ValidatingWebhookConfiguration" || resKind == "MutatingWebhookConfiguration" {
					result = append(result, res)
				}
			}

			// Ensure service exists
			services := ensureService(objects, info.serviceName, namespace, info.port)
			for _, svc := range services {
				svcName := svc.GetName()
				if !processedServices[svcName] {
					result = append(result, svc)
					processedServices[svcName] = true
				}
			}
		} else if kind == "Service" {
			// Track existing services to avoid duplicates
			processedServices[obj.GetName()] = true
			result = append(result, obj)
		} else {
			result = append(result, obj)
		}
	}

	return result, nil
}

type webhookInfo struct {
	obj         *unstructured.Unstructured
	kind        string
	serviceName string
	namespace   string
	port        int32
}

// findWebhooks scans for webhook configurations in the objects.
func findWebhooks(objects []*unstructured.Unstructured) []*webhookInfo {
	var webhooks []*webhookInfo

	for _, obj := range objects {
		kind := obj.GetKind()
		if kind != "ValidatingWebhookConfiguration" && kind != "MutatingWebhookConfiguration" {
			continue
		}

		info := extractWebhookInfo(obj, kind)
		if info != nil {
			webhooks = append(webhooks, info)
		}
	}

	return webhooks
}

// extractWebhookInfo extracts service info from webhook configuration.
func extractWebhookInfo(obj *unstructured.Unstructured, kind string) *webhookInfo {
	webhooks, found, err := unstructured.NestedSlice(obj.Object, "webhooks")
	if !found || err != nil || len(webhooks) == 0 {
		return nil
	}

	// Get the first webhook's clientConfig
	webhook, ok := webhooks[0].(map[string]any)
	if !ok {
		return nil
	}

	clientConfig, found, err := unstructured.NestedMap(webhook, "clientConfig")
	if !found || err != nil {
		return nil
	}

	service, found, err := unstructured.NestedMap(clientConfig, "service")
	if !found || err != nil {
		return nil
	}

	serviceName, _, _ := unstructured.NestedString(service, "name")
	serviceNamespace, _, _ := unstructured.NestedString(service, "namespace")
	port, _, _ := unstructured.NestedInt64(service, "port")

	if serviceName == "" {
		return nil
	}

	return &webhookInfo{
		obj:         obj,
		kind:        kind,
		serviceName: serviceName,
		namespace:   serviceNamespace,
		port:        int32(port),
	}
}

// ensureService verifies or creates a Service for the webhook.
func ensureService(
	objects []*unstructured.Unstructured,
	serviceName string,
	namespace string,
	port int32,
) []*unstructured.Unstructured {
	// Check if service already exists
	for _, obj := range objects {
		if obj.GetKind() == "Service" && obj.GetName() == serviceName {
			// Service exists, verify/update port if needed
			return updateServicePort(obj, port)
		}
	}

	// Service doesn't exist, create it
	targetPort := findTargetPort(objects, serviceName, port)
	svc := createService(serviceName, namespace, port, targetPort)
	return []*unstructured.Unstructured{svc}
}

// updateServicePort updates service port if it doesn't match.
func updateServicePort(svc *unstructured.Unstructured, expectedPort int32) []*unstructured.Unstructured {
	ports, found, err := unstructured.NestedSlice(svc.Object, "spec", "ports")
	if !found || err != nil || len(ports) == 0 {
		// No ports defined, add one
		newPort := map[string]any{
			"name":       "https",
			"port":       int64(expectedPort),
			"targetPort": int64(expectedPort),
			"protocol":   "TCP",
		}
		_ = unstructured.SetNestedSlice(svc.Object, []any{newPort}, "spec", "ports")
		return []*unstructured.Unstructured{svc}
	}

	// Check if port matches
	firstPort, ok := ports[0].(map[string]any)
	if ok {
		currentPort, _, _ := unstructured.NestedInt64(firstPort, "port")
		if int32(currentPort) != expectedPort {
			_ = unstructured.SetNestedField(firstPort, int64(expectedPort), "port")
			ports[0] = firstPort
			_ = unstructured.SetNestedSlice(svc.Object, ports, "spec", "ports")
		}
	}

	return []*unstructured.Unstructured{svc}
}

// findTargetPort finds the target port from deployment.
func findTargetPort(objects []*unstructured.Unstructured, serviceName string, defaultPort int32) int32 {
	// Extract deployment name from service name (convention: <deployment>-webhook-service)
	deploymentName := serviceName
	suffix := "-webhook-service"
	if len(serviceName) > len(suffix) && serviceName[len(serviceName)-len(suffix):] == suffix {
		deploymentName = serviceName[:len(serviceName)-len(suffix)]
	}

	for _, obj := range objects {
		if obj.GetKind() != "Deployment" || obj.GetName() != deploymentName {
			continue
		}

		// Convert to typed Deployment
		var deployment appsv1.Deployment
		if err := kube.FromUnstructured(obj, &deployment); err != nil {
			continue
		}

		// Extract container port from first container
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			container := deployment.Spec.Template.Spec.Containers[0]
			if len(container.Ports) > 0 {
				return container.Ports[0].ContainerPort
			}
		}
	}

	return defaultPort
}

// createService creates a new Service resource.
func createService(serviceName string, namespace string, port int32, targetPort int32) *unstructured.Unstructured {
	// Extract deployment name from service name (if it follows the convention)
	selector := serviceName
	suffix := "-webhook-service"
	if len(serviceName) > len(suffix) && serviceName[len(serviceName)-len(suffix):] == suffix {
		selector = serviceName[:len(serviceName)-len(suffix)]
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       port,
					TargetPort: intstr.FromInt32(targetPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": selector,
			},
		},
	}

	u, err := kube.ToUnstructured(svc)
	if err != nil {
		return nil
	}

	return u
}
