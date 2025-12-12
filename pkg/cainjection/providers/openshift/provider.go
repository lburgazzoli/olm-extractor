package openshift

// Package openshift provides OpenShift service CA based injection for webhooks.

import (
	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Provider implements CAProvider for OpenShift service CA.
type Provider struct{}

// New creates a new OpenShift CA provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "openshift"
}

// ConfigureWebhook configures a webhook with OpenShift service CA injection.
func (p *Provider) ConfigureWebhook(
	webhook *unstructured.Unstructured,
	serviceName string,
	namespace string,
) ([]*unstructured.Unstructured, error) {
	configMapName := serviceName + "-ca"

	// Create ConfigMap for CA bundle
	configMap := createCAConfigMap(configMapName, serviceName, namespace)

	// Add annotation to webhook to reference the ConfigMap
	annotatedWebhook, err := addAnnotation(webhook, configMapName, namespace)
	if err != nil {
		return nil, err
	}

	return []*unstructured.Unstructured{configMap, annotatedWebhook}, nil
}

// createCAConfigMap creates a ConfigMap for OpenShift service CA injection.
func createCAConfigMap(configMapName string, serviceName string, namespace string) *unstructured.Unstructured {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Annotations: map[string]string{
				"service.beta.openshift.io/inject-cabundle": "true",
			},
		},
	}

	u, err := kube.ToUnstructured(cm)
	if err != nil {
		return nil
	}

	return u
}

// addAnnotation adds OpenShift CA bundle reference to webhook.
func addAnnotation(webhook *unstructured.Unstructured, configMapName string, namespace string) (*unstructured.Unstructured, error) {
	kind := webhook.GetKind()

	switch kind {
	case "ValidatingWebhookConfiguration":
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &vwc); err != nil {
			return nil, err
		}

		if vwc.Annotations == nil {
			vwc.Annotations = make(map[string]string)
		}
		vwc.Annotations["service.beta.openshift.io/inject-cabundle"] = "true"
		vwc.Annotations["service.ca.openshift.io/inject-cabundle-from"] = namespace + "/" + configMapName

		return kube.ToUnstructured(&vwc)

	case "MutatingWebhookConfiguration":
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := kube.FromUnstructured(webhook, &mwc); err != nil {
			return nil, err
		}

		if mwc.Annotations == nil {
			mwc.Annotations = make(map[string]string)
		}
		mwc.Annotations["service.beta.openshift.io/inject-cabundle"] = "true"
		mwc.Annotations["service.ca.openshift.io/inject-cabundle-from"] = namespace + "/" + configMapName

		return kube.ToUnstructured(&mwc)

	default:
		return webhook, nil
	}
}
