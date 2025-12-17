package filter_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/olm-extractor/pkg/filter"

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

	matches, err := f.Matches(obj)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())
}

func TestMatches_ExcludeOnly(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{}, []string{".kind == \"Secret\""})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	matches, err := f.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())
}

func TestMatches_IncludeOnly(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	matches, err := f.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}

func TestMatches_MultipleIncludesActAsOR(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\"", ".kind == \"Service\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}
	configMap := &unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "config"}}}

	matches, err := f.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(configMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}

func TestMatches_MultipleExcludesActAsOR(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{}, []string{".kind == \"Secret\"", ".kind == \"ConfigMap\""})
	g.Expect(err).ToNot(HaveOccurred())

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	secret := &unstructured.Unstructured{Object: map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "secret"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}
	configMap := &unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "config"}}}

	matches, err := f.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(secret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(configMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}

func TestMatches_ExcludePriorityOverInclude(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".kind == \"Deployment\""}, []string{".metadata.name == \"excluded-app\""})
	g.Expect(err).ToNot(HaveOccurred())

	includedDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}
	excludedDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "excluded-app"}}}
	service := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc"}}}

	matches, err := f.Matches(includedDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(excludedDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}

func TestMatches_NestedFieldAccess(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".metadata.namespace == \"default\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	defaultDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app", "namespace": "default"}}}
	kubeSystemDeployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app2", "namespace": "kube-system"}}}
	defaultService := &unstructured.Unstructured{Object: map[string]any{"kind": "Service", "metadata": map[string]any{"name": "svc", "namespace": "default"}}}

	matches, err := f.Matches(defaultDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(kubeSystemDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(defaultService)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())
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

	matches, err := f.Matches(highReplicaDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())

	matches, err = f.Matches(lowReplicaDeployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(service)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}

func TestMatches_OnlyBooleanTrueMatches(t *testing.T) {
	g := NewWithT(t)

	deployment := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "app"}}}

	// This should NOT match because .kind returns "Deployment" (a string), not true
	f1, err := filter.New([]string{".kind"}, []string{})
	g.Expect(err).ToNot(HaveOccurred())
	matches, err := f1.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	// This SHOULD match because == returns boolean true
	f2, err := filter.New([]string{".kind == \"Deployment\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())
	matches, err = f2.Matches(deployment)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeTrue())
}

func TestMatches_ErrorsAreReturned(t *testing.T) {
	g := NewWithT(t)

	f, err := filter.New([]string{".metadata.name == \"foo\""}, []string{})
	g.Expect(err).ToNot(HaveOccurred())

	deploymentWithoutMetadata := &unstructured.Unstructured{Object: map[string]any{"kind": "Deployment"}}
	serviceWithoutMetadata := &unstructured.Unstructured{Object: map[string]any{"kind": "Service"}}

	// jq queries that access missing fields return null, not errors
	// These should succeed (return false) without errors
	matches, err := f.Matches(deploymentWithoutMetadata)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())

	matches, err = f.Matches(serviceWithoutMetadata)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(matches).To(BeFalse())
}
