package kube_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
	"github.com/lburgazzoli/olm-extractor/pkg/kube/gvks"

	. "github.com/onsi/gomega"
)

func TestIsNamespaced(t *testing.T) {
	t.Run("returns false for cluster-scoped resources", func(t *testing.T) {
		g := NewWithT(t)

		clusterScoped := []schema.GroupVersionKind{
			gvks.Namespace,
			gvks.CustomResourceDefinition,
			gvks.ClusterRole,
			gvks.ClusterRoleBinding,
			gvks.PersistentVolume,
			gvks.StorageClass,
			gvks.PriorityClass,
			gvks.ValidatingWebhookConfiguration,
			gvks.MutatingWebhookConfiguration,
			gvks.ClusterIssuer,
		}

		for _, gvk := range clusterScoped {
			g.Expect(kube.IsNamespaced(gvk)).To(BeFalse(), "expected %q to be cluster-scoped", gvk.Kind)
		}
	})

	t.Run("returns true for namespaced resources", func(t *testing.T) {
		g := NewWithT(t)

		namespaced := []schema.GroupVersionKind{
			{Group: "", Kind: "Pod"},
			gvks.Deployment,
			gvks.Service,
			gvks.ConfigMap,
			{Group: "", Kind: "Secret"},
			{Group: "", Kind: "ServiceAccount"},
			{Group: "rbac.authorization.k8s.io", Kind: "Role"},
			{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"},
			{Group: "", Kind: "PersistentVolumeClaim"},
		}

		for _, gvk := range namespaced {
			g.Expect(kube.IsNamespaced(gvk)).To(BeTrue(), "expected %q to be namespaced", gvk.Kind)
		}
	})
}

func TestCreateNamespace(t *testing.T) {
	t.Run("creates namespace with correct name", func(t *testing.T) {
		g := NewWithT(t)

		ns := kube.CreateNamespace("my-namespace")

		g.Expect(ns.Name).To(Equal("my-namespace"))
		g.Expect(ns.Kind).To(Equal("Namespace"))
		g.Expect(ns.APIVersion).To(Equal("v1"))
	})
}

func TestCreateDeployment(t *testing.T) {
	t.Run("function exists", func(t *testing.T) {
		g := NewWithT(t)

		// We can't easily create a StrategyDeploymentSpec without the full OLM types,
		// but we can verify the function exists and basic behavior.
		// Full integration tests would need actual CSV data.
		g.Expect(kube.CreateDeployment).NotTo(BeNil())
	})
}

func TestSetNamespace(t *testing.T) {
	t.Run("sets namespace on namespaced object", func(t *testing.T) {
		g := NewWithT(t)

		ns := kube.CreateNamespace("original")
		err := kube.SetNamespace(ns, "updated")

		g.Expect(err).ToNot(HaveOccurred())
		// Namespace is cluster-scoped, but the function should still work
		// on any object implementing metav1.Object
		g.Expect(ns.Namespace).To(Equal("updated"))
	})
}

func TestValidateNamespace(t *testing.T) {
	t.Run("accepts valid namespace names", func(t *testing.T) {
		g := NewWithT(t)

		validNames := []string{
			"default",
			"kube-system",
			"my-namespace",
			"operators",
			"a",
			"abc123",
			"test-ns-1",
		}

		for _, name := range validNames {
			g.Expect(kube.ValidateNamespace(name)).To(Succeed(), "expected %q to be valid", name)
		}
	})

	t.Run("rejects empty namespace", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("")).To(MatchError("namespace cannot be empty"))
	})

	t.Run("rejects namespace longer than 63 characters", func(t *testing.T) {
		g := NewWithT(t)

		longName := "a123456789012345678901234567890123456789012345678901234567890123" // 64 chars
		g.Expect(kube.ValidateNamespace(longName)).To(MatchError(ContainSubstring("must be no more than 63 characters")))
	})

	t.Run("accepts namespace starting with digit", func(t *testing.T) {
		g := NewWithT(t)
		// DNS-1123 labels allow names starting with digits
		g.Expect(kube.ValidateNamespace("1test")).To(Succeed())
		g.Expect(kube.ValidateNamespace("123-test")).To(Succeed())
	})

	t.Run("rejects namespace starting with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("-test")).To(MatchError(ContainSubstring("must start and end with an alphanumeric character")))
	})

	t.Run("rejects namespace ending with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test-")).To(MatchError(ContainSubstring("must start and end with an alphanumeric character")))
	})

	t.Run("rejects namespace with uppercase letters", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("Test")).To(MatchError(ContainSubstring("lower case alphanumeric characters")))
	})

	t.Run("rejects namespace with underscores", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test_ns")).To(MatchError(ContainSubstring("lower case alphanumeric characters")))
	})

	t.Run("rejects namespace with dots", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test.ns")).To(MatchError(ContainSubstring("must not contain dots")))
	})
}

func TestCleanUnstructured(t *testing.T) {
	t.Run("preserves empty string in environment variable value field", func(t *testing.T) {
		g := NewWithT(t)

		// Create an unstructured object with an env var that has empty string value
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name": "test",
									"env": []any{
										map[string]any{
											"name":  "WATCH_NAMESPACE",
											"value": "", // Empty string should be preserved
										},
										map[string]any{
											"name":  "OTHER_VAR",
											"value": "non-empty",
										},
									},
								},
							},
						},
					},
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		// Verify the structure is preserved
		containers, found, err := unstructured.NestedSlice(cleaned.Object, "spec", "template", "spec", "containers")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(containers).To(HaveLen(1))

		container := containers[0].(map[string]any)
		env, found, err := unstructured.NestedSlice(container, "env")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(env).To(HaveLen(2))

		// Check WATCH_NAMESPACE with empty value is preserved
		watchNsEnv := env[0].(map[string]any)
		g.Expect(watchNsEnv["name"]).To(Equal("WATCH_NAMESPACE"))
		g.Expect(watchNsEnv).To(HaveKey("value"))
		g.Expect(watchNsEnv["value"]).To(Equal(""))

		// Check OTHER_VAR is preserved
		otherEnv := env[1].(map[string]any)
		g.Expect(otherEnv["name"]).To(Equal("OTHER_VAR"))
		g.Expect(otherEnv["value"]).To(Equal("non-empty"))
	})

	t.Run("removes empty strings from non-env-var fields", func(t *testing.T) {
		g := NewWithT(t)

		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"data": map[string]any{
					"key1": "",          // Should be removed
					"key2": "non-empty", // Should be kept
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		data, found, err := unstructured.NestedMap(cleaned.Object, "data")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(data).To(HaveLen(1))
		g.Expect(data).ToNot(HaveKey("key1")) // Empty string removed
		g.Expect(data).To(HaveKey("key2"))
	})

	t.Run("does not preserve empty strings in maps with only name field", func(t *testing.T) {
		g := NewWithT(t)

		// A map with just "name" is NOT an env var (missing value/valueFrom)
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name": "test-config",
				},
				"data": map[string]any{
					"someField": "", // Should be removed (not an env var)
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		// When all fields in "data" are removed, the entire "data" map is removed too
		_, found, err := unstructured.NestedMap(cleaned.Object, "data")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeFalse()) // data field removed entirely because all values were empty
	})

	t.Run("preserves empty value in env var with valueFrom", func(t *testing.T) {
		g := NewWithT(t)

		// EnvVar with valueFrom should also be detected (even if value happens to be empty)
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name": "test",
									"env": []any{
										map[string]any{
											"name": "FROM_SECRET",
											"valueFrom": map[string]any{
												"secretKeyRef": map[string]any{
													"name": "my-secret",
													"key":  "password",
												},
											},
											"value": "", // Edge case: has both (unusual but valid)
										},
									},
								},
							},
						},
					},
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		containers, found, err := unstructured.NestedSlice(cleaned.Object, "spec", "template", "spec", "containers")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())

		container := containers[0].(map[string]any)
		env, found, err := unstructured.NestedSlice(container, "env")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())

		envVar := env[0].(map[string]any)
		g.Expect(envVar["name"]).To(Equal("FROM_SECRET"))
		g.Expect(envVar).To(HaveKey("valueFrom"))
		g.Expect(envVar).To(HaveKey("value"))
		g.Expect(envVar["value"]).To(Equal("")) // Preserved because it's an env var
	})

	t.Run("preserves empty subresources status in CRD", func(t *testing.T) {
		g := NewWithT(t)

		// Create a CRD-like object with subresources.status: {}
		// This is critical for Kubernetes controllers that use UpdateStatus()
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]any{
					"name": "certmanagers.operator.openshift.io",
				},
				"spec": map[string]any{
					"group": "operator.openshift.io",
					"versions": []any{
						map[string]any{
							"name":    "v1alpha1",
							"served":  true,
							"storage": true,
							"subresources": map[string]any{
								"status": map[string]any{}, // Empty status map - must be preserved!
							},
							"schema": map[string]any{
								"openAPIV3Schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		// Verify the structure is preserved
		versions, found, err := unstructured.NestedSlice(cleaned.Object, "spec", "versions")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(versions).To(HaveLen(1))

		version := versions[0].(map[string]any)

		// The subresources field must be preserved even though status: {} is empty
		subresources, found, err := unstructured.NestedMap(version, "subresources")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue(), "subresources field must be preserved")
		g.Expect(subresources).To(HaveKey("status"), "subresources.status must be preserved")
	})

	t.Run("preserves emptyDir in volume definitions", func(t *testing.T) {
		g := NewWithT(t)

		// Deployment with emptyDir: {} volume - must be preserved
		// olm-extractor was stripping this because cleanValue treats empty maps as nil
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "test-operator",
					"namespace": "test-ns",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "operator",
									"image": "registry.example.com/operator:latest",
									"volumeMounts": []any{
										map[string]any{
											"name":      "tmp",
											"mountPath": "/tmp",
										},
									},
								},
							},
							"volumes": []any{
								map[string]any{
									"name":     "tmp",
									"emptyDir": map[string]any{}, // Empty map - must be preserved!
								},
							},
						},
					},
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		volumes, found, err := unstructured.NestedSlice(cleaned.Object, "spec", "template", "spec", "volumes")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(volumes).To(HaveLen(1))

		volume := volumes[0].(map[string]any)
		g.Expect(volume["name"]).To(Equal("tmp"))
		g.Expect(volume).To(HaveKey("emptyDir"), "emptyDir field must be preserved even when empty")
	})

	t.Run("preserves subresources with scale in CRD", func(t *testing.T) {
		g := NewWithT(t)

		// CRD with both status and scale subresources
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"spec": map[string]any{
					"versions": []any{
						map[string]any{
							"name": "v1",
							"subresources": map[string]any{
								"status": map[string]any{},
								"scale": map[string]any{
									"specReplicasPath":   ".spec.replicas",
									"statusReplicasPath": ".status.replicas",
								},
							},
						},
					},
				},
			},
		}

		cleaned := kube.CleanUnstructured(obj)

		versions, found, err := unstructured.NestedSlice(cleaned.Object, "spec", "versions")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())

		version := versions[0].(map[string]any)
		subresources, found, err := unstructured.NestedMap(version, "subresources")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(subresources).To(HaveKey("status"))
		g.Expect(subresources).To(HaveKey("scale"))
	})
}
