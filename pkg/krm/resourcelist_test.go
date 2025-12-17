package krm_test

import (
	"bytes"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/lburgazzoli/olm-extractor/pkg/krm"

	. "github.com/onsi/gomega"
)

func TestReadResourceList(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*krm.ResourceList)
	}{
		{
			name: "valid Extractor functionConfig - bundle mode",
			input: `apiVersion: config.kubernetes.io/v1
kind: ResourceList
items: []
functionConfig:
  apiVersion: olm.lburgazzoli.github.io/v1alpha1
  kind: Extractor
  metadata:
    name: test-operator
  spec:
    source: quay.io/example/operator:v1.0.0
    namespace: operators
`,
			wantErr: false,
			check: func(rl *krm.ResourceList) {
				g.Expect(rl.APIVersion).To(Equal("config.kubernetes.io/v1"))
				g.Expect(rl.Kind).To(Equal("ResourceList"))
				g.Expect(rl.FunctionConfig).ToNot(BeNil())
				g.Expect(rl.FunctionConfig.GetKind()).To(Equal("Extractor"))
			},
		},
		{
			name: "valid Extractor functionConfig - catalog mode",
			input: `apiVersion: config.kubernetes.io/v1
kind: ResourceList
items: []
functionConfig:
  apiVersion: olm.lburgazzoli.github.io/v1alpha1
  kind: Extractor
  metadata:
    name: test-operator
  spec:
    source: prometheus:0.56.0
    catalog:
      source: quay.io/catalog:latest
      channel: stable
    namespace: monitoring
`,
			wantErr: false,
			check: func(rl *krm.ResourceList) {
				g.Expect(rl.FunctionConfig).ToNot(BeNil())
				g.Expect(rl.FunctionConfig.GetKind()).To(Equal("Extractor"))
			},
		},
		{
			name: "invalid APIVersion",
			input: `apiVersion: v1
kind: ResourceList
items: []
`,
			wantErr: true,
		},
		{
			name: "invalid Kind",
			input: `apiVersion: config.kubernetes.io/v1
kind: SomethingElse
items: []
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reader := bytes.NewReader([]byte(tt.input))

			rl, err := krm.ReadResourceList(reader)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rl).ToNot(BeNil())

			if tt.check != nil {
				tt.check(rl)
			}
		})
	}
}

func TestWriteResourceList(t *testing.T) {
	g := NewWithT(t)

	rl := krm.NewResourceList()
	rl.Items = []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test-cm",
					"namespace": "default",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := krm.WriteResourceList(&buf, rl)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(buf.String()).To(ContainSubstring("config.kubernetes.io/v1"))
	g.Expect(buf.String()).To(ContainSubstring("ResourceList"))
	g.Expect(buf.String()).To(ContainSubstring("test-cm"))
}

func TestExtractFunctionConfig(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		rl      *krm.ResourceList
		wantErr bool
		check   func(*krm.FunctionConfig)
	}{
		{
			name: "Extractor bundle mode configuration",
			rl: &krm.ResourceList{
				FunctionConfig: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "olm.lburgazzoli.github.io/v1alpha1",
						"kind":       "Extractor",
						"metadata": map[string]any{
							"name": "test-operator",
						},
						"spec": map[string]any{
							"source":    "quay.io/example/operator:v1.0.0",
							"namespace": "operators",
						},
					},
				},
			},
			wantErr: false,
			check: func(fc *krm.FunctionConfig) {
				g.Expect(fc.Kind).To(Equal("Extractor"))
				g.Expect(fc.ExtractorConfig).ToNot(BeNil())
				g.Expect(fc.ExtractorConfig.Spec.Source).To(Equal("quay.io/example/operator:v1.0.0"))
				g.Expect(fc.ExtractorConfig.Spec.Namespace).To(Equal("operators"))
				g.Expect(fc.ExtractorConfig.Spec.Catalog).To(BeNil())
			},
		},
		{
			name: "Extractor catalog mode configuration",
			rl: &krm.ResourceList{
				FunctionConfig: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "olm.lburgazzoli.github.io/v1alpha1",
						"kind":       "Extractor",
						"metadata": map[string]any{
							"name": "test-operator",
						},
						"spec": map[string]any{
							"source":    "prometheus:0.56.0",
							"namespace": "monitoring",
							"catalog": map[string]any{
								"source":  "quay.io/catalog:latest",
								"channel": "stable",
							},
						},
					},
				},
			},
			wantErr: false,
			check: func(fc *krm.FunctionConfig) {
				g.Expect(fc.Kind).To(Equal("Extractor"))
				g.Expect(fc.ExtractorConfig).ToNot(BeNil())
				g.Expect(fc.ExtractorConfig.Spec.Source).To(Equal("prometheus:0.56.0"))
				g.Expect(fc.ExtractorConfig.Spec.Namespace).To(Equal("monitoring"))
				g.Expect(fc.ExtractorConfig.Spec.Catalog).ToNot(BeNil())
				g.Expect(fc.ExtractorConfig.Spec.Catalog.Source).To(Equal("quay.io/catalog:latest"))
				g.Expect(fc.ExtractorConfig.Spec.Catalog.Channel).To(Equal("stable"))
			},
		},
		{
			name: "missing functionConfig",
			rl: &krm.ResourceList{
				FunctionConfig: nil,
			},
			wantErr: true,
		},
		{
			name: "unsupported kind",
			rl: &krm.ResourceList{
				FunctionConfig: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fc, err := krm.ExtractFunctionConfig(tt.rl)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())

				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(fc).ToNot(BeNil())

			if tt.check != nil {
				tt.check(fc)
			}
		})
	}
}

func TestNewResourceList(t *testing.T) {
	g := NewWithT(t)

	rl := krm.NewResourceList()

	g.Expect(rl.APIVersion).To(Equal("config.kubernetes.io/v1"))
	g.Expect(rl.Kind).To(Equal("ResourceList"))
	g.Expect(rl.Items).ToNot(BeNil())
	g.Expect(rl.Items).To(BeEmpty())
}

func TestResourceListAddResults(t *testing.T) {
	g := NewWithT(t)

	rl := krm.NewResourceList()

	rl.AddErrorf("test error")
	rl.AddWarningf("test warning")
	rl.AddInfof("test info")

	g.Expect(rl.Results).To(HaveLen(3))
	g.Expect(rl.Results[0].Severity).To(Equal(krm.SeverityError))
	g.Expect(rl.Results[0].Message).To(Equal("test error"))
	g.Expect(rl.Results[1].Severity).To(Equal(krm.SeverityWarning))
	g.Expect(rl.Results[1].Message).To(Equal("test warning"))
	g.Expect(rl.Results[2].Severity).To(Equal(krm.SeverityInfo))
	g.Expect(rl.Results[2].Message).To(Equal("test info"))
}

func TestResourceListAddResultsWithFormatting(t *testing.T) {
	g := NewWithT(t)

	rl := krm.NewResourceList()

	rl.AddErrorf("failed to process %s: %v", "resource", "some error")
	rl.AddWarningf("warning for %s at line %d", "file.yaml", 42)
	rl.AddInfof("processed %d items successfully", 10)

	g.Expect(rl.Results).To(HaveLen(3))
	g.Expect(rl.Results[0].Severity).To(Equal(krm.SeverityError))
	g.Expect(rl.Results[0].Message).To(Equal("failed to process resource: some error"))
	g.Expect(rl.Results[1].Severity).To(Equal(krm.SeverityWarning))
	g.Expect(rl.Results[1].Message).To(Equal("warning for file.yaml at line 42"))
	g.Expect(rl.Results[2].Severity).To(Equal(krm.SeverityInfo))
	g.Expect(rl.Results[2].Message).To(Equal("processed 10 items successfully"))
}
