package kube

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
)

const (
	// DefaultWebhookPortName is the standard port name for webhook services.
	DefaultWebhookPortName = "https"
)

// DeploymentInfo contains port and selector information from a deployment.
type DeploymentInfo struct {
	Port     int32
	Selector map[string]string
}

// EnsureService verifies or creates a service for a webhook.
// Returns a slice of services (typically one) that should be added to the object list.
func EnsureService(
	objects []*unstructured.Unstructured,
	serviceName string,
	namespace string,
	port int32,
	webhookServiceSuffix string,
) ([]*unstructured.Unstructured, error) {
	// Check if service already exists
	for _, obj := range objects {
		if Is(obj, gvks.Service, serviceName) {
			// Service exists, verify/update port if needed
			return UpdateServicePort(obj, port)
		}
	}

	// Service doesn't exist, create it using deployment info
	info := FindDeploymentInfo(objects, serviceName, port, webhookServiceSuffix)
	svc, err := CreateService(serviceName, namespace, port, info.Port, info.Selector, DefaultWebhookPortName)
	if err != nil {
		return nil, fmt.Errorf("failed to create service %s: %w", serviceName, err)
	}

	return []*unstructured.Unstructured{svc}, nil
}

// UpdateServicePort updates the service port if it doesn't match expected.
// Returns the updated service as a single-element slice.
func UpdateServicePort(svc *unstructured.Unstructured, expectedPort int32) ([]*unstructured.Unstructured, error) {
	var service corev1.Service
	if err := FromUnstructured(svc, &service); err != nil {
		return nil, fmt.Errorf("failed to convert service: %w", err)
	}

	// Check if ports exist
	if len(service.Spec.Ports) == 0 {
		// No ports defined, add one
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       DefaultWebhookPortName,
				Port:       expectedPort,
				TargetPort: intstr.FromInt32(expectedPort),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	} else if service.Spec.Ports[0].Port != expectedPort {
		// Update existing port
		service.Spec.Ports[0].Port = expectedPort
	}

	// Convert back to unstructured
	updated, err := ToUnstructured(&service)
	if err != nil {
		return nil, fmt.Errorf("failed to convert service to unstructured: %w", err)
	}

	return []*unstructured.Unstructured{updated}, nil
}

// CreateService creates a new Service resource with given parameters.
// If no selector is provided, it derives a default selector from the service name.
func CreateService(
	serviceName string,
	namespace string,
	port int32,
	targetPort int32,
	selector map[string]string,
	portName string,
) (*unstructured.Unstructured, error) {
	if len(selector) == 0 {
		selector = map[string]string{
			"app.kubernetes.io/name": serviceName,
		}
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gvks.Service.GroupVersion().String(),
			Kind:       gvks.Service.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       portName,
					Port:       port,
					TargetPort: intstr.FromInt32(targetPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: selector,
		},
	}

	u, err := ToUnstructured(svc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert service to unstructured: %w", err)
	}

	return u, nil
}

// FindDeploymentInfo extracts port and selector information from a deployment.
// The deploymentName is derived from the serviceName by removing the webhookServiceSuffix.
func FindDeploymentInfo(
	objects []*unstructured.Unstructured,
	serviceName string,
	defaultPort int32,
	webhookServiceSuffix string,
) DeploymentInfo {
	// Extract deployment name from service name (convention: <deployment>-webhook-service)
	deploymentName := serviceName
	if len(serviceName) > len(webhookServiceSuffix) && serviceName[len(serviceName)-len(webhookServiceSuffix):] == webhookServiceSuffix {
		deploymentName = serviceName[:len(serviceName)-len(webhookServiceSuffix)]
	}

	for _, obj := range objects {
		if !Is(obj, gvks.Deployment, deploymentName) {
			continue
		}

		// Convert to typed Deployment
		var deployment appsv1.Deployment
		if err := FromUnstructured(obj, &deployment); err != nil {
			continue
		}

		info := DeploymentInfo{
			Port: defaultPort,
		}

		// Extract selector from deployment
		if deployment.Spec.Selector != nil {
			info.Selector = deployment.Spec.Selector.MatchLabels
		}

		// Extract container port from first container
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			container := deployment.Spec.Template.Spec.Containers[0]
			if len(container.Ports) > 0 {
				info.Port = container.Ports[0].ContainerPort
			}
		}

		return info
	}

	return DeploymentInfo{Port: defaultPort, Selector: nil}
}
