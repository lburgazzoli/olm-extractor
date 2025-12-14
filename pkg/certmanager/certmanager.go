package certmanager

import (
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// webhookServiceSuffix is the conventional suffix for webhook service names.
	webhookServiceSuffix = "-webhook-service"

	// certNameSuffix is appended to service names to create certificate names.
	certNameSuffix = "-cert"

	// tlsSecretSuffix is appended to service names to create TLS secret names.
	tlsSecretSuffix = "-tls"

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
	webhookObjects, processedServiceNames, err := processWebhooks(objects, webhooks, namespace, cfg.IssuerName, cfg.IssuerKind)
	if err != nil {
		return nil, err
	}

	// Add remaining non-webhook objects (excluding processed services)
	remainingObjects := kube.Find(objects, func(obj *unstructured.Unstructured) bool {
		if kube.IsWebhookConfiguration(obj) {
			return false
		}
		if kube.IsKind(obj, gvks.Service) && processedServiceNames[obj.GetName()] {
			return false
		}

		return true
	})

	return append(webhookObjects, remainingObjects...), nil
}

// processWebhooks handles webhook processing and returns the configured webhook objects.
// It tracks processed services to avoid duplicates when shared by multiple webhooks.
// Returns the webhook objects and a map of processed service names.
func processWebhooks(
	objects []*unstructured.Unstructured,
	webhooks []*unstructured.Unstructured,
	namespace string,
	issuerName string,
	issuerKind string,
) ([]*unstructured.Unstructured, map[string]bool, error) {
	result := make([]*unstructured.Unstructured, 0, len(webhooks)*expectedObjectsPerWebhook)
	processedServices := make(map[string]bool)

	for _, obj := range webhooks {
		info := kube.ExtractWebhookServiceInfo(obj)
		if info == nil {
			result = append(result, obj)

			continue
		}

		// Create Certificate and configure webhook
		certName := info.ServiceName + certNameSuffix

		// Check if certificate already added
		if !hasCertificate(result, certName) {
			cert, err := createCertificate(certName, info.ServiceName, namespace, issuerName, issuerKind)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create certificate %s: %w", certName, err)
			}
			result = append(result, cert)
		}

		// Add cert-manager annotation to webhook
		annotationValue := namespace + "/" + certName
		annotatedWebhook, err := kube.AddWebhookAnnotation(obj, certmanagerv1.WantInjectAnnotation, annotationValue)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to configure webhook %s: %w", obj.GetName(), err)
		}
		result = append(result, annotatedWebhook)

		// Ensure service exists (only add once if shared by multiple webhooks)
		if !processedServices[info.ServiceName] {
			services, err := kube.EnsureService(objects, info.ServiceName, namespace, info.Port, webhookServiceSuffix)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to ensure service %s for webhook %s: %w", info.ServiceName, obj.GetName(), err)
			}
			result = append(result, services...)
			processedServices[info.ServiceName] = true
		}
	}

	return result, processedServices, nil
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
