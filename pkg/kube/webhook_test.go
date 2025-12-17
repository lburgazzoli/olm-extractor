package kube_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"

	. "github.com/onsi/gomega"
)

func TestExtractWebhookServiceInfo_ValidatingWebhook(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "my-webhook",
			},
			"webhooks": []any{
				map[string]any{
					"name": "validate.example.com",
					"clientConfig": map[string]any{
						"service": map[string]any{
							"name":      "my-service",
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	info := kube.ExtractWebhookServiceInfo(webhook)

	g.Expect(info).ToNot(BeNil())
	g.Expect(info.ServiceName).To(Equal("my-service"))
	g.Expect(info.Namespace).To(Equal("default"))
	g.Expect(info.Port).To(Equal(int32(443)))
}

func TestExtractWebhookServiceInfo_MutatingWebhook(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "MutatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "my-webhook",
			},
			"webhooks": []any{
				map[string]any{
					"name": "mutate.example.com",
					"clientConfig": map[string]any{
						"service": map[string]any{
							"name":      "my-mutating-service",
							"namespace": "test-ns",
							"port":      int64(8443),
						},
					},
				},
			},
		},
	}

	info := kube.ExtractWebhookServiceInfo(webhook)

	g.Expect(info).ToNot(BeNil())
	g.Expect(info.ServiceName).To(Equal("my-mutating-service"))
	g.Expect(info.Namespace).To(Equal("test-ns"))
	g.Expect(info.Port).To(Equal(int32(8443)))
}

func TestExtractWebhookServiceInfo_WebhookWithoutService(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "my-webhook",
			},
			"webhooks": []any{
				map[string]any{
					"name": "validate.example.com",
					"clientConfig": map[string]any{
						"url": "https://example.com/validate",
					},
				},
			},
		},
	}

	info := kube.ExtractWebhookServiceInfo(webhook)

	g.Expect(info).To(BeNil())
}

func TestExtractWebhookServiceInfo_EmptyWebhooks(t *testing.T) {
	g := NewWithT(t)

	webhook := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "my-webhook",
			},
			"webhooks": []any{},
		},
	}

	info := kube.ExtractWebhookServiceInfo(webhook)

	g.Expect(info).To(BeNil())
}

func TestExtractWebhookServiceInfo_NotWebhook(t *testing.T) {
	g := NewWithT(t)

	notWebhook := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name": "my-service",
			},
		},
	}

	info := kube.ExtractWebhookServiceInfo(notWebhook)

	g.Expect(info).To(BeNil())
}
