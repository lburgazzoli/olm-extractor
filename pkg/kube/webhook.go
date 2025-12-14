package kube

import (
	"fmt"

	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// WebhookInfo contains extracted service information from webhook configurations.
type WebhookInfo struct {
	ServiceName string
	Namespace   string
	Port        int32
}

// ExtractWebhookServiceInfo extracts service configuration from webhook objects.
// Returns nil if webhook doesn't reference a service.
func ExtractWebhookServiceInfo(obj *unstructured.Unstructured) *WebhookInfo {
	if IsKind(obj, gvks.ValidatingWebhookConfiguration) {
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := FromUnstructured(obj, &vwc); err != nil {
			return nil
		}

		if len(vwc.Webhooks) == 0 || vwc.Webhooks[0].ClientConfig.Service == nil {
			return nil
		}

		svc := vwc.Webhooks[0].ClientConfig.Service

		return &WebhookInfo{
			ServiceName: svc.Name,
			Namespace:   svc.Namespace,
			Port:        *svc.Port,
		}
	}

	if IsKind(obj, gvks.MutatingWebhookConfiguration) {
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := FromUnstructured(obj, &mwc); err != nil {
			return nil
		}

		if len(mwc.Webhooks) == 0 || mwc.Webhooks[0].ClientConfig.Service == nil {
			return nil
		}

		svc := mwc.Webhooks[0].ClientConfig.Service

		return &WebhookInfo{
			ServiceName: svc.Name,
			Namespace:   svc.Namespace,
			Port:        *svc.Port,
		}
	}

	return nil
}

// AddWebhookAnnotation adds an annotation to webhook configurations.
// Returns a new unstructured object with the annotation added.
func AddWebhookAnnotation(webhook *unstructured.Unstructured, key string, value string) (*unstructured.Unstructured, error) {
	switch webhook.GroupVersionKind() {
	case gvks.ValidatingWebhookConfiguration:
		var vwc admissionregistrationv1.ValidatingWebhookConfiguration
		if err := FromUnstructured(webhook, &vwc); err != nil {
			return nil, fmt.Errorf("failed to convert validating webhook: %w", err)
		}

		if vwc.Annotations == nil {
			vwc.Annotations = make(map[string]string)
		}
		vwc.Annotations[key] = value

		u, err := ToUnstructured(&vwc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert validating webhook to unstructured: %w", err)
		}

		return u, nil

	case gvks.MutatingWebhookConfiguration:
		var mwc admissionregistrationv1.MutatingWebhookConfiguration
		if err := FromUnstructured(webhook, &mwc); err != nil {
			return nil, fmt.Errorf("failed to convert mutating webhook: %w", err)
		}

		if mwc.Annotations == nil {
			mwc.Annotations = make(map[string]string)
		}
		mwc.Annotations[key] = value

		u, err := ToUnstructured(&mwc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert mutating webhook to unstructured: %w", err)
		}

		return u, nil

	default:
		return webhook, nil
	}
}
