package certmanager

// Package certmanager provides cert-manager based CA injection for webhooks.

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultIssuerName = "selfsigned-issuer"
)

// Provider implements CAProvider for cert-manager.
type Provider struct{}

// New creates a new cert-manager CA provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "cert-manager"
}

// ConfigureWebhook configures a webhook with cert-manager CA injection.
func (p *Provider) ConfigureWebhook(
	webhook *unstructured.Unstructured,
	serviceName string,
	namespace string,
) ([]*unstructured.Unstructured, error) {
	certName := serviceName + "-cert"

	// Create Certificate
	cert := createCertificate(certName, serviceName, namespace)

	// Add annotation to webhook
	annotatedWebhook, err := addAnnotation(webhook, certName, namespace)
	if err != nil {
		return nil, err
	}

	return []*unstructured.Unstructured{cert, annotatedWebhook}, nil
}

// createCertificate creates a cert-manager Certificate resource.
func createCertificate(certName string, serviceName string, namespace string) *unstructured.Unstructured {
	secretName := serviceName + "-tls"

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
				Kind: "Issuer",
				Name: defaultIssuerName,
			},
		},
	}

	u, err := kube.ToUnstructured(cert)
	if err != nil {
		return nil
	}

	return u
}

// addAnnotation adds cert-manager injection annotation to webhook.
func addAnnotation(webhook *unstructured.Unstructured, certName string, namespace string) (*unstructured.Unstructured, error) {
	kind := webhook.GetKind()
	annotationValue := namespace + "/" + certName

	switch kind {
	case "ValidatingWebhookConfiguration":
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &vwc); err != nil {
			return nil, err
		}

		if vwc.Annotations == nil {
			vwc.Annotations = make(map[string]string)
		}
		vwc.Annotations["cert-manager.io/inject-ca-from"] = annotationValue

		return kube.ToUnstructured(&vwc)

	case "MutatingWebhookConfiguration":
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &mwc); err != nil {
			return nil, err
		}

		if mwc.Annotations == nil {
			mwc.Annotations = make(map[string]string)
		}
		mwc.Annotations["cert-manager.io/inject-ca-from"] = annotationValue

		return kube.ToUnstructured(&mwc)

	default:
		return webhook, nil
	}
}
