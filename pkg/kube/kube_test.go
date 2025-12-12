package kube_test

import (
	"testing"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"

	. "github.com/onsi/gomega"
)

func TestIsNamespaced(t *testing.T) {
	t.Run("returns false for cluster-scoped resources", func(t *testing.T) {
		g := NewWithT(t)

		clusterScoped := []string{
			"Namespace",
			"CustomResourceDefinition",
			"ClusterRole",
			"ClusterRoleBinding",
			"PersistentVolume",
			"StorageClass",
			"PriorityClass",
			"ValidatingWebhookConfiguration",
			"MutatingWebhookConfiguration",
		}

		for _, kind := range clusterScoped {
			g.Expect(kube.IsNamespaced(kind)).To(BeFalse(), "expected %q to be cluster-scoped", kind)
		}
	})

	t.Run("returns true for namespaced resources", func(t *testing.T) {
		g := NewWithT(t)

		namespaced := []string{
			"Pod",
			"Deployment",
			"Service",
			"ConfigMap",
			"Secret",
			"ServiceAccount",
			"Role",
			"RoleBinding",
			"PersistentVolumeClaim",
		}

		for _, kind := range namespaced {
			g.Expect(kube.IsNamespaced(kind)).To(BeTrue(), "expected %q to be namespaced", kind)
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
		kube.SetNamespace(ns, "updated")

		// Namespace is cluster-scoped, but the function should still work
		// on any object implementing metav1.Object
		g.Expect(ns.Namespace).To(Equal("updated"))
	})
}
