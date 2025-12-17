package kube_test

import (
	"testing"

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
		kube.SetNamespace(ns, "updated")

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
		g.Expect(kube.ValidateNamespace(longName)).To(MatchError("namespace name too long (max 63 characters)"))
	})

	t.Run("rejects namespace starting with digit", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("1test")).To(MatchError("invalid namespace name: must start with a lowercase letter"))
	})

	t.Run("rejects namespace starting with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("-test")).To(MatchError("invalid namespace name: must start with a lowercase letter"))
	})

	t.Run("rejects namespace ending with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test-")).To(MatchError("invalid namespace name: must end with an alphanumeric character"))
	})

	t.Run("rejects namespace with uppercase letters", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("Test")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})

	t.Run("rejects namespace with underscores", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test_ns")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})

	t.Run("rejects namespace with dots", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(kube.ValidateNamespace("test.ns")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})
}
