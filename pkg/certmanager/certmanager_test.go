package certmanager_test

import (
	"testing"

	"github.com/lburgazzoli/olm-extractor/pkg/certmanager"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/gomega"
)

func TestConfigure_NoWebhooks(t *testing.T) {
	g := NewWithT(t)

	objects := []*unstructured.Unstructured{
		{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}},
		{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}},
	}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
}

func TestConfigure_ValidatingWebhook(t *testing.T) {
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

	objects := []*unstructured.Unstructured{webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(3)) // certificate + webhook + service

	// Check Certificate was created
	var foundCert *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.Certificate.Kind {
			foundCert = obj

			break
		}
	}
	g.Expect(foundCert).ToNot(BeNil())
	g.Expect(foundCert.GetName()).To(Equal("my-service-cert"))

	// Check webhook has annotation
	var foundWebhook *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.ValidatingWebhookConfiguration.Kind {
			foundWebhook = obj

			break
		}
	}

	g.Expect(foundWebhook).ToNot(BeNil())
	annotations := foundWebhook.GetAnnotations()
	g.Expect(annotations).To(HaveKey("cert-manager.io/inject-ca-from"))
	g.Expect(annotations["cert-manager.io/inject-ca-from"]).To(Equal("default/my-service-cert"))
}

func TestConfigure_MutatingWebhook(t *testing.T) {
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
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(3)) // certificate + webhook + service

	// Check Certificate was created
	var foundCert *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.Certificate.Kind {
			foundCert = obj

			break
		}
	}
	g.Expect(foundCert).ToNot(BeNil())
	g.Expect(foundCert.GetName()).To(Equal("my-mutating-service-cert"))

	// Check webhook has annotation
	var foundWebhook *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.MutatingWebhookConfiguration.Kind {
			foundWebhook = obj

			break
		}
	}

	g.Expect(foundWebhook).ToNot(BeNil())
	annotations := foundWebhook.GetAnnotations()
	g.Expect(annotations).To(HaveKey("cert-manager.io/inject-ca-from"))
	g.Expect(annotations["cert-manager.io/inject-ca-from"]).To(Equal("default/my-mutating-service-cert"))
}

func TestConfigure_ServiceAlreadyExists(t *testing.T) {
	g := NewWithT(t)

	service := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      "my-service",
				"namespace": "default",
			},
			"spec": map[string]any{
				"ports": []any{
					map[string]any{
						"name":       "https",
						"port":       int64(443),
						"targetPort": int64(9443),
						"protocol":   "TCP",
					},
				},
			},
		},
	}

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

	objects := []*unstructured.Unstructured{service, webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(3)) // certificate + service + webhook
}

func TestConfigure_ServiceWithDeployment(t *testing.T) {
	g := NewWithT(t)

	deployment := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-service",
				"namespace": "default",
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "webhook",
								"ports": []any{
									map[string]any{
										"containerPort": int64(9443),
									},
								},
							},
						},
					},
				},
			},
		},
	}

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
							"name":      "my-service-webhook-service",
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{deployment, webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(4)) // certificate + deployment + webhook + service

	// Find created service and verify targetPort matches deployment
	var foundService *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.Service.Kind && obj.GetName() == "my-service-webhook-service" {
			foundService = obj

			break
		}
	}

	g.Expect(foundService).ToNot(BeNil())
	ports, found, _ := unstructured.NestedSlice(foundService.Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	targetPort, _, _ := unstructured.NestedInt64(port, "targetPort")
	g.Expect(targetPort).To(Equal(int64(9443)))
}

func TestConfigure_MultipleWebhooks(t *testing.T) {
	g := NewWithT(t)

	webhook1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "webhook1",
			},
			"webhooks": []any{
				map[string]any{
					"name": "validate1.example.com",
					"clientConfig": map[string]any{
						"service": map[string]any{
							"name":      "service1",
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	webhook2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "MutatingWebhookConfiguration",
			"metadata": map[string]any{
				"name": "webhook2",
			},
			"webhooks": []any{
				map[string]any{
					"name": "mutate.example.com",
					"clientConfig": map[string]any{
						"service": map[string]any{
							"name":      "service2",
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{webhook1, webhook2}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(6)) // 2 certificates + 2 webhooks + 2 services

	// Verify both webhooks have annotations
	webhookCount := 0
	for _, obj := range result {
		if obj.GetKind() == gvks.ValidatingWebhookConfiguration.Kind || obj.GetKind() == gvks.MutatingWebhookConfiguration.Kind {
			webhookCount++
			annotations := obj.GetAnnotations()
			g.Expect(annotations).To(HaveKey("cert-manager.io/inject-ca-from"))
		}
	}
	g.Expect(webhookCount).To(Equal(2))
}

func TestConfigure_WebhookWithoutServiceInfo(t *testing.T) {
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

	objects := []*unstructured.Unstructured{webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1)) // just the webhook, no changes

	// Webhook should not have annotation since it doesn't use a service
	annotations := result[0].GetAnnotations()
	g.Expect(annotations).ToNot(HaveKey("cert-manager.io/inject-ca-from"))
}

func TestConfigure_DeploymentWithCustomLabels(t *testing.T) {
	g := NewWithT(t)

	deployment := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "my-controller",
				"namespace": "default",
			},
			"spec": map[string]any{
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app":       "custom-app",
						"component": "webhook",
						"tier":      "control-plane",
					},
				},
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "webhook",
								"ports": []any{
									map[string]any{
										"containerPort": int64(8443),
									},
								},
							},
						},
					},
				},
			},
		},
	}

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
							"name":      "my-controller-webhook-service",
							"namespace": "default",
							"port":      int64(443),
						},
					},
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{deployment, webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(4)) // certificate + deployment + webhook + service

	// Find created service and verify it uses deployment's actual selector
	var foundService *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.Service.Kind && obj.GetName() == "my-controller-webhook-service" {
			foundService = obj

			break
		}
	}

	g.Expect(foundService).ToNot(BeNil())

	// Verify service selector matches deployment's matchLabels
	selector, found, _ := unstructured.NestedStringMap(foundService.Object, "spec", "selector")
	g.Expect(found).To(BeTrue())
	g.Expect(selector).To(HaveKeyWithValue("app", "custom-app"))
	g.Expect(selector).To(HaveKeyWithValue("component", "webhook"))
	g.Expect(selector).To(HaveKeyWithValue("tier", "control-plane"))

	// Verify targetPort was extracted from deployment
	ports, found, _ := unstructured.NestedSlice(foundService.Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	targetPort, _, _ := unstructured.NestedInt64(port, "targetPort")
	g.Expect(targetPort).To(Equal(int64(8443)))
}

func TestConfigure_ServiceWithExistingPort(t *testing.T) {
	g := NewWithT(t)

	service := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      "my-service",
				"namespace": "default",
			},
			"spec": map[string]any{
				"ports": []any{
					map[string]any{
						"name":       "https",
						"port":       int64(8080),
						"targetPort": int64(8080),
						"protocol":   "TCP",
					},
				},
			},
		},
	}

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

	objects := []*unstructured.Unstructured{service, webhook}

	cfg := certmanager.Config{
		Enabled:    true,
		IssuerName: "test-issuer",
		IssuerKind: "ClusterIssuer",
	}
	result, err := certmanager.Configure(objects, "default", cfg)

	g.Expect(err).ToNot(HaveOccurred())

	// Find service and verify port was updated to match webhook requirement
	var foundService *unstructured.Unstructured
	for _, obj := range result {
		if obj.GetKind() == gvks.Service.Kind && obj.GetName() == "my-service" {
			foundService = obj

			break
		}
	}

	g.Expect(foundService).ToNot(BeNil())

	ports, found, _ := unstructured.NestedSlice(foundService.Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	// Port should be updated to 443
	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(443)))
}
