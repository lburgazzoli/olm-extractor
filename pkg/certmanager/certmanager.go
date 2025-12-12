package certmanager

import (
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// webhookServiceSuffix is the conventional suffix for webhook service names.
	webhookServiceSuffix = "-webhook-service"

	// certNameSuffix is appended to service names to create certificate names.
	certNameSuffix = "-cert"

	// tlsSecretSuffix is appended to service names to create TLS secret names.
	tlsSecretSuffix = "-tls"

	// webhookServicePortName is the standard port name for webhook services.
	webhookServicePortName = "https"

	// certManagerInjectCAAnnotation is the annotation for cert-manager CA injection.
	certManagerInjectCAAnnotation = "cert-manager.io/inject-ca-from"
)

// Configure analyzes filtered resources and configures cert-manager CA injection for webhooks.
// It creates Certificate resources and ensures services exist for webhooks.
func Configure(objects []*unstructured.Unstructured, namespace string, issuerName string, issuerKind string) ([]*unstructured.Unstructured, error) {
	webhooks := findWebhooks(objects)
	if len(webhooks) == 0 {
		return objects, nil
	}

	result := make([]*unstructured.Unstructured, 0, len(objects))
	processedServices := make(map[string]bool)
	addedCertificates := make(map[string]bool)

	// First pass: process all webhooks and their services
	for _, obj := range objects {
		if !kube.IsKind(obj, gvks.ValidatingWebhookConfiguration) && !kube.IsKind(obj, gvks.MutatingWebhookConfiguration) {
			continue
		}

		info := extractWebhookInfo(obj)
		if info == nil {
			result = append(result, obj)
			continue
		}

		// Create Certificate and configure webhook
		certName := info.serviceName + certNameSuffix

		// Create Certificate resource
		cert := createCertificate(certName, info.serviceName, namespace, issuerName, issuerKind)
		if !addedCertificates[certName] {
			result = append(result, cert)
			addedCertificates[certName] = true
		}

		// Add cert-manager annotation to webhook
		annotatedWebhook, err := addCertManagerAnnotation(obj, certName, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to configure webhook %s: %w", obj.GetName(), err)
		}
		result = append(result, annotatedWebhook)

		// Ensure service exists
		services, err := ensureService(objects, info.serviceName, namespace, info.port)
		if err != nil {
			return nil, fmt.Errorf("failed to ensure service %s for webhook %s: %w", info.serviceName, obj.GetName(), err)
		}
		for _, svc := range services {
			svcName := svc.GetName()
			if !processedServices[svcName] {
				result = append(result, svc)
				processedServices[svcName] = true
			}
		}
	}

	// Second pass: add remaining objects
	for _, obj := range objects {
		if kube.IsKind(obj, gvks.ValidatingWebhookConfiguration) || kube.IsKind(obj, gvks.MutatingWebhookConfiguration) {
			// Already processed in first pass
			continue
		}

		if kube.IsKind(obj, gvks.Service) {
			// Only add service if not already processed by webhook handling
			if !processedServices[obj.GetName()] {
				result = append(result, obj)
			}
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
		if !kube.IsKind(obj, gvks.ValidatingWebhookConfiguration) && !kube.IsKind(obj, gvks.MutatingWebhookConfiguration) {
			continue
		}

		info := extractWebhookInfo(obj)
		if info != nil {
			webhooks = append(webhooks, info)
		}
	}

	return webhooks
}

// extractWebhookInfo extracts service info from webhook configuration.
func extractWebhookInfo(obj *unstructured.Unstructured) *webhookInfo {
	if kube.IsKind(obj, gvks.ValidatingWebhookConfiguration) {
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := kube.FromUnstructured(obj, &vwc); err != nil {
			return nil
		}

		if len(vwc.Webhooks) == 0 || vwc.Webhooks[0].ClientConfig.Service == nil {
			return nil
		}

		svc := vwc.Webhooks[0].ClientConfig.Service
		return &webhookInfo{
			obj:         obj,
			kind:        obj.GetKind(),
			serviceName: svc.Name,
			namespace:   svc.Namespace,
			port:        *svc.Port,
		}
	}

	if kube.IsKind(obj, gvks.MutatingWebhookConfiguration) {
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := kube.FromUnstructured(obj, &mwc); err != nil {
			return nil
		}

		if len(mwc.Webhooks) == 0 || mwc.Webhooks[0].ClientConfig.Service == nil {
			return nil
		}

		svc := mwc.Webhooks[0].ClientConfig.Service
		return &webhookInfo{
			obj:         obj,
			kind:        obj.GetKind(),
			serviceName: svc.Name,
			namespace:   svc.Namespace,
			port:        *svc.Port,
		}
	}

	return nil
}

// ensureService verifies or creates a Service for the webhook.
func ensureService(
	objects []*unstructured.Unstructured,
	serviceName string,
	namespace string,
	port int32,
) ([]*unstructured.Unstructured, error) {
	// Check if service already exists
	for _, obj := range objects {
		if kube.Is(obj, gvks.Service, serviceName) {
			// Service exists, verify/update port if needed
			return updateServicePort(obj, port)
		}
	}

	// Service doesn't exist, create it using deployment info
	info := findDeploymentInfo(objects, serviceName, port)
	svc := createService(serviceName, namespace, port, info.port, info.selector)
	return []*unstructured.Unstructured{svc}, nil
}

// updateServicePort updates service port if it doesn't match.
func updateServicePort(svc *unstructured.Unstructured, expectedPort int32) ([]*unstructured.Unstructured, error) {
	var service corev1.Service
	if err := kube.FromUnstructured(svc, &service); err != nil {
		return nil, fmt.Errorf("failed to convert service: %w", err)
	}

	// Check if ports exist
	if len(service.Spec.Ports) == 0 {
		// No ports defined, add one
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       webhookServicePortName,
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
	updated, err := kube.ToUnstructured(&service)
	if err != nil {
		return nil, fmt.Errorf("failed to convert service to unstructured: %w", err)
	}

	return []*unstructured.Unstructured{updated}, nil
}

type deploymentInfo struct {
	port     int32
	selector map[string]string
}

// findDeploymentInfo finds the target port and selector from deployment.
func findDeploymentInfo(objects []*unstructured.Unstructured, serviceName string, defaultPort int32) deploymentInfo {
	// Extract deployment name from service name (convention: <deployment>-webhook-service)
	deploymentName := serviceName
	if len(serviceName) > len(webhookServiceSuffix) && serviceName[len(serviceName)-len(webhookServiceSuffix):] == webhookServiceSuffix {
		deploymentName = serviceName[:len(serviceName)-len(webhookServiceSuffix)]
	}

	for _, obj := range objects {
		if !kube.Is(obj, gvks.Deployment, deploymentName) {
			continue
		}

		// Convert to typed Deployment
		var deployment appsv1.Deployment
		if err := kube.FromUnstructured(obj, &deployment); err != nil {
			continue
		}

		info := deploymentInfo{
			port: defaultPort,
		}

		// Extract selector from deployment
		if deployment.Spec.Selector != nil {
			info.selector = deployment.Spec.Selector.MatchLabels
		}

		// Extract container port from first container
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			container := deployment.Spec.Template.Spec.Containers[0]
			if len(container.Ports) > 0 {
				info.port = container.Ports[0].ContainerPort
			}
		}

		return info
	}

	return deploymentInfo{port: defaultPort, selector: nil}
}

// createService creates a new Service resource.
func createService(
	serviceName string,
	namespace string,
	port int32,
	targetPort int32,
	selector map[string]string,
) *unstructured.Unstructured {
	// If no selector provided, derive from service name convention
	if len(selector) == 0 {
		selectorValue := serviceName
		if len(serviceName) > len(webhookServiceSuffix) && serviceName[len(serviceName)-len(webhookServiceSuffix):] == webhookServiceSuffix {
			selectorValue = serviceName[:len(serviceName)-len(webhookServiceSuffix)]
		}
		selector = map[string]string{
			"app.kubernetes.io/name": selectorValue,
		}
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
					Name:       webhookServicePortName,
					Port:       port,
					TargetPort: intstr.FromInt32(targetPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: selector,
		},
	}

	u, err := kube.ToUnstructured(svc)
	if err != nil {
		return nil
	}

	return u
}

// createCertificate creates a cert-manager Certificate resource.
func createCertificate(certName string, serviceName string, namespace string, issuerName string, issuerKind string) *unstructured.Unstructured {
	secretName := serviceName + tlsSecretSuffix

	cert := &certmanagerv1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      certName,
			Namespace: namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: secretName,
			DNSNames: []string{
				serviceName + "." + namespace + ".svc",
				serviceName + "." + namespace + ".svc.cluster.local",
			},
			IssuerRef: cmmeta.ObjectReference{
				Kind: issuerKind,
				Name: issuerName,
			},
		},
	}

	u, err := kube.ToUnstructured(cert)
	if err != nil {
		return nil
	}

	return u
}

// addCertManagerAnnotation adds cert-manager injection annotation to webhook.
func addCertManagerAnnotation(webhook *unstructured.Unstructured, certName string, namespace string) (*unstructured.Unstructured, error) {
	annotationValue := namespace + "/" + certName

	switch webhook.GroupVersionKind() {
	case gvks.ValidatingWebhookConfiguration:
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &vwc); err != nil {
			return nil, err
		}

		if vwc.Annotations == nil {
			vwc.Annotations = make(map[string]string)
		}
		vwc.Annotations[certManagerInjectCAAnnotation] = annotationValue

		return kube.ToUnstructured(&vwc)

	case gvks.MutatingWebhookConfiguration:
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &mwc); err != nil {
			return nil, err
		}

		if mwc.Annotations == nil {
			mwc.Annotations = make(map[string]string)
		}
		mwc.Annotations[certManagerInjectCAAnnotation] = annotationValue

		return kube.ToUnstructured(&mwc)

	default:
		return webhook, nil
	}
}
