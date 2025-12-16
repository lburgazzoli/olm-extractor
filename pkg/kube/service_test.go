package kube_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"

	. "github.com/onsi/gomega"
)

func TestCreateService(t *testing.T) {
	g := NewWithT(t)

	svc, err := kube.CreateService(
		"my-service",
		"default",
		443,
		9443,
		map[string]string{"app": "test"},
		"https",
	)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(svc).ToNot(BeNil())
	g.Expect(svc.GetName()).To(Equal("my-service"))
	g.Expect(svc.GetNamespace()).To(Equal("default"))

	selector, found, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	g.Expect(found).To(BeTrue())
	g.Expect(selector).To(HaveKeyWithValue("app", "test"))

	ports, found, _ := unstructured.NestedSlice(svc.Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(443)))

	targetPort, _, _ := unstructured.NestedInt64(port, "targetPort")
	g.Expect(targetPort).To(Equal(int64(9443)))

	portName, _, _ := unstructured.NestedString(port, "name")
	g.Expect(portName).To(Equal("https"))
}

func TestCreateService_DefaultSelector(t *testing.T) {
	g := NewWithT(t)

	svc, err := kube.CreateService(
		"my-service",
		"default",
		443,
		443,
		nil,
		"https",
	)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(svc).ToNot(BeNil())

	selector, found, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	g.Expect(found).To(BeTrue())
	g.Expect(selector).To(HaveKeyWithValue("app.kubernetes.io/name", "my-service"))
}

func TestUpdateServicePort_AddPort(t *testing.T) {
	g := NewWithT(t)

	svc := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      "my-service",
				"namespace": "default",
			},
			"spec": map[string]any{
				"ports": []any{},
			},
		},
	}

	result, err := kube.UpdateServicePort(svc, 8080)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1))

	ports, found, _ := unstructured.NestedSlice(result[0].Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(8080)))
}

func TestUpdateServicePort_UpdateExisting(t *testing.T) {
	g := NewWithT(t)

	svc := &unstructured.Unstructured{
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

	result, err := kube.UpdateServicePort(svc, 8443)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1))

	ports, found, _ := unstructured.NestedSlice(result[0].Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(8443)))
}

func TestUpdateServicePort_NoChange(t *testing.T) {
	g := NewWithT(t)

	svc := &unstructured.Unstructured{
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

	result, err := kube.UpdateServicePort(svc, 443)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1))

	ports, found, _ := unstructured.NestedSlice(result[0].Object, "spec", "ports")
	g.Expect(found).To(BeTrue())
	g.Expect(ports).To(HaveLen(1))

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(443)))
}

func TestFindDeploymentInfo_WithDeployment(t *testing.T) {
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
						"app":       "my-app",
						"component": "controller",
					},
				},
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "controller",
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

	objects := []*unstructured.Unstructured{deployment}

	info := kube.FindDeploymentInfo(objects, "my-controller-webhook-service", 443, "-webhook-service")

	g.Expect(info.Port).To(Equal(int32(8443)))
	g.Expect(info.Selector).To(HaveKeyWithValue("app", "my-app"))
	g.Expect(info.Selector).To(HaveKeyWithValue("component", "controller"))
}

func TestFindDeploymentInfo_NoDeployment(t *testing.T) {
	g := NewWithT(t)

	objects := []*unstructured.Unstructured{}

	info := kube.FindDeploymentInfo(objects, "my-service", 443, "-webhook-service")

	g.Expect(info.Port).To(Equal(int32(443)))
	g.Expect(info.Selector).To(BeNil())
}

func TestFindDeploymentInfo_NoContainerPorts(t *testing.T) {
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
						"app": "my-app",
					},
				},
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "controller",
							},
						},
					},
				},
			},
		},
	}

	objects := []*unstructured.Unstructured{deployment}

	info := kube.FindDeploymentInfo(objects, "my-controller-webhook-service", 9443, "-webhook-service")

	g.Expect(info.Port).To(Equal(int32(9443)))
	g.Expect(info.Selector).To(HaveKeyWithValue("app", "my-app"))
}

func TestEnsureService_ServiceExists(t *testing.T) {
	g := NewWithT(t)

	existingService := &unstructured.Unstructured{
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

	objects := []*unstructured.Unstructured{existingService}

	result, err := kube.EnsureService(objects, "my-service", "default", 443, "-webhook-service")

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1))
	g.Expect(result[0].GetName()).To(Equal("my-service"))

	ports, found, _ := unstructured.NestedSlice(result[0].Object, "spec", "ports")
	g.Expect(found).To(BeTrue())

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(443)))
}

func TestEnsureService_ServiceDoesNotExist(t *testing.T) {
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
						"app": "my-app",
					},
				},
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "controller",
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

	objects := []*unstructured.Unstructured{deployment}

	result, err := kube.EnsureService(objects, "my-controller-webhook-service", "default", 443, "-webhook-service")

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(HaveLen(1))
	g.Expect(result[0].GetName()).To(Equal("my-controller-webhook-service"))
	g.Expect(result[0].GetNamespace()).To(Equal("default"))

	ports, found, _ := unstructured.NestedSlice(result[0].Object, "spec", "ports")
	g.Expect(found).To(BeTrue())

	port, ok := ports[0].(map[string]any)
	g.Expect(ok).To(BeTrue())

	portNum, _, _ := unstructured.NestedInt64(port, "port")
	g.Expect(portNum).To(Equal(int64(443)))

	targetPort, _, _ := unstructured.NestedInt64(port, "targetPort")
	g.Expect(targetPort).To(Equal(int64(9443)))

	selector, found, _ := unstructured.NestedStringMap(result[0].Object, "spec", "selector")
	g.Expect(found).To(BeTrue())
	g.Expect(selector).To(HaveKeyWithValue("app", "my-app"))
}
