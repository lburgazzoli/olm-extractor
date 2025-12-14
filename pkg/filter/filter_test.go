package filter_test

import (
	"testing"

	"github.com/lburgazzoli/olm-extractor/pkg/filter"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/gomega"
)

func TestNew_ValidExpressions(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New(
		[]string{".kind == \"Deployment\"", ".metadata.name == \"foo\""},
		[]string{".kind == \"Secret\""},
	)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f).ToNot(BeNil())
}

func TestNew_InvalidIncludeExpression(t *testing.T) {
	g := NewWithT(t)

	_, err := filter.New(
		[]string{".kind == invalid syntax"},
		[]string{},
	)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid include expression"))
}

func TestNew_InvalidExcludeExpression(t *testing.T) {
	g := NewWithT(t)

	_, err := filter.New(
		[]string{},
		[]string{".kind == invalid syntax"},
	)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid exclude expression"))
}

func TestMatches_NoFilters(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	obj := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}

	g.Expect(f.Matches(obj)).To(BeTrue())
}

func TestMatches_ExcludeOnly(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{}, []string{".kind == \"Secret\""})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	g.Expect(f.Matches(deployment)).To(BeTrue())
	g.Expect(f.Matches(secret)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeTrue())
}

func TestMatches_IncludeOnly(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	g.Expect(f.Matches(deployment)).To(BeTrue())
	g.Expect(f.Matches(secret)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeFalse())
}

func TestMatches_MultipleIncludesActAsOR(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\"", ".kind == \"Service\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}
	configMap := &unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "config"}}}

	g.Expect(f.Matches(deployment)).To(BeTrue())
	g.Expect(f.Matches(secret)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeTrue())
	g.Expect(f.Matches(configMap)).To(BeFalse())
}

func TestMatches_MultipleExcludesActAsOR(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{}, []string{".kind == \"Secret\"", ".kind == \"ConfigMap\""})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}
	configMap := &unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "config"}}}

	g.Expect(f.Matches(deployment)).To(BeTrue())
	g.Expect(f.Matches(secret)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeTrue())
	g.Expect(f.Matches(configMap)).To(BeFalse())
}

func TestMatches_ExcludePriorityOverInclude(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\""}, []string{".metadata.name == \"excluded-app\""})
	g.Expect(err).ToNot(HaveOccurred())

	includedDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	excludedDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "excluded-app"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	g.Expect(f.Matches(includedDeployment)).To(BeTrue())
	g.Expect(f.Matches(excludedDeployment)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeFalse())
}

func TestMatches_NestedFieldAccess(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".metadata.namespace == \"default\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	defaultDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app", "namespace": "default"}}}
	kubeSystemDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app2", "namespace": "kube-system"}}}
	defaultService := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc", "namespace": "default"}}}

	g.Expect(f.Matches(defaultDeployment)).To(BeTrue())
	g.Expect(f.Matches(kubeSystemDeployment)).To(BeFalse())
	g.Expect(f.Matches(defaultService)).To(BeTrue())
}

func TestMatches_ComplexJQExpression(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\" and .spec.replicas > 1"}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	highReplicaDeployment := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment",
		"spec": map[string]any{
			"replicas": float64(3),
		},
	}}
	lowReplicaDeployment := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment",
		"spec": map[string]any{
			"replicas": float64(1),
		},
	}}
	service := &unstructured.Unstructured{Object: map[string]any{
		"kind": "Service",
	}}

	g.Expect(f.Matches(highReplicaDeployment)).To(BeTrue())
	g.Expect(f.Matches(lowReplicaDeployment)).To(BeFalse())
	g.Expect(f.Matches(service)).To(BeFalse())
}

func TestMatches_OnlyBooleanTrueMatches(t *testing.T) {
	g := NewWithT(t)

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}

	// This should NOT match because .kind returns "Deployment" (a string), not true
	f1, err := filter.New([]string{".kind"}, []string{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f1.Matches(deployment)).To(BeFalse())

	// This SHOULD match because == returns boolean true
	f2, err := filter.New([]string{".kind == \"Deployment\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(f2.Matches(deployment)).To(BeTrue())
}

func TestMatches_ErrorsDoNotMatch(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".metadata.name == \"foo\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deploymentWithoutMetadata := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}}
	serviceWithoutMetadata := &unstructured.Unstructured{Object: map[string]any{"kind": "Service"}}

	// This expression will error on objects without .metadata.name
	// but should not crash, just not match those objects
	g.Expect(f.Matches(deploymentWithoutMetadata)).To(BeFalse())
	g.Expect(f.Matches(serviceWithoutMetadata)).To(BeFalse())
}
