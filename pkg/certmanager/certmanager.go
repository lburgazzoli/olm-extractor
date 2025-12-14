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

	// processedAnnotation marks objects that have been processed by Configure.
	processedAnnotation = "olm-extractor.lburgazzoli.github.io/processed"

	// expectedObjectsPerWebhook is the estimated number of objects generated per webhook
	// (webhook + certificate + service).
	expectedObjectsPerWebhook = 3
)

// Config holds configuration for cert-manager integration.
type Config struct {
	Enabled    bool   `mapstructure:"cert-manager-enabled"`
	IssuerName string `mapstructure:"cert-manager-issuer-name"`
	IssuerKind string `mapstructure:"cert-manager-issuer-kind"`
}

// Configure analyzes filtered resources and configures cert-manager CA injection for webhooks.
// It creates Certificate resources and ensures services exist for webhooks.
func Configure(objects []*unstructured.Unstructured, namespace string, cfg Config) ([]*unstructured.Unstructured, error) {
	webhooks := kube.Find(objects, kube.IsWebhookConfiguration)
	if len(webhooks) == 0 {
		return objects, nil
	}

	// Process all webhooks and their services
	webhookObjects, err := processWebhooks(objects, webhooks, namespace, cfg.IssuerName, cfg.IssuerKind)
	if err != nil {
		return nil, err
	}

	// Add remaining non-webhook objects (excluding processed services)
	remainingObjects := kube.Find(objects, func(obj *unstructured.Unstructured) bool {
		return !kube.IsWebhookConfiguration(obj) && !kube.HasAnnotation(obj, processedAnnotation)
	})

	return append(webhookObjects, remainingObjects...), nil
}

// processWebhooks handles webhook processing and returns the configured webhook objects.
// It marks processed services with an annotation to avoid duplicates.
func processWebhooks(
	objects []*unstructured.Unstructured,
	webhooks []*unstructured.Unstructured,
	namespace string,
	issuerName string,
	issuerKind string,
) ([]*unstructured.Unstructured, error) {
	result := make([]*unstructured.Unstructured, 0, len(webhooks)*expectedObjectsPerWebhook)

	for _, obj := range webhooks {
		info := extractWebhookInfo(obj)
		if info == nil {
			result = append(result, obj)

			continue
		}

		// Create Certificate and configure webhook
		certName := info.serviceName + certNameSuffix

		// Check if certificate already added
		if !hasCertificate(result, certName) {
			cert, err := createCertificate(certName, info.serviceName, namespace, issuerName, issuerKind)
			if err != nil {
				return nil, fmt.Errorf("failed to create certificate %s: %w", certName, err)
			}
			result = append(result, cert)
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
			if !kube.HasAnnotation(svc, processedAnnotation) {
				kube.SetAnnotation(svc, processedAnnotation, "true")
				result = append(result, svc)
			}
		}
	}

	return result, nil
}

// hasCertificate checks if a certificate with the given name exists in the result.
func hasCertificate(objects []*unstructured.Unstructured, certName string) bool {
	for _, obj := range objects {
		if kube.IsKind(obj, gvks.Certificate) && obj.GetName() == certName {
			return true
		}
	}

	return false
}

type webhookInfo struct {
	obj         *unstructured.Unstructured
	kind        string
	serviceName string
	namespace   string
	port        int32
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
			services, err := updateServicePort(obj, port)
			if err != nil {
				return nil, err
			}
			// Mark original service as processed to avoid duplicates
			kube.SetAnnotation(obj, processedAnnotation, "true")

			return services, nil
		}
	}

	// Service doesn't exist, create it using deployment info
	info := findDeploymentInfo(objects, serviceName, port)
	svc, err := createService(serviceName, namespace, port, info.port, info.selector)
	if err != nil {
		return nil, fmt.Errorf("failed to create service %s: %w", serviceName, err)
	}

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
) (*unstructured.Unstructured, error) {
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
		return nil, fmt.Errorf("failed to convert service to unstructured: %w", err)
	}

	return u, nil
}

// createCertificate creates a cert-manager Certificate resource.
func createCertificate(certName string, serviceName string, namespace string, issuerName string, issuerKind string) (*unstructured.Unstructured, error) {
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
		return nil, fmt.Errorf("failed to convert certificate to unstructured: %w", err)
	}

	return u, nil
}

// addCertManagerAnnotation adds cert-manager injection annotation to webhook.
func addCertManagerAnnotation(webhook *unstructured.Unstructured, certName string, namespace string) (*unstructured.Unstructured, error) {
	annotationValue := namespace + "/" + certName

	switch webhook.GroupVersionKind() {
	case gvks.ValidatingWebhookConfiguration:
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &vwc); err != nil {
			return nil, fmt.Errorf("failed to convert validating webhook: %w", err)
		}

		if vwc.Annotations == nil {
			vwc.Annotations = make(map[string]string)
		}
		vwc.Annotations[certManagerInjectCAAnnotation] = annotationValue

		u, err := kube.ToUnstructured(&vwc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert validating webhook to unstructured: %w", err)
		}

		return u, nil

	case gvks.MutatingWebhookConfiguration:
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &mwc); err != nil {
			return nil, fmt.Errorf("failed to convert mutating webhook: %w", err)
		}

		if mwc.Annotations == nil {
			mwc.Annotations = make(map[string]string)
		}
		mwc.Annotations[certManagerInjectCAAnnotation] = annotationValue

		u, err := kube.ToUnstructured(&mwc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mutating webhook to unstructured: %w", err)
		}

		return u, nil

	default:
		return webhook, nil
	}
}
