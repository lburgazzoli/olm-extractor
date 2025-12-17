package render

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/olm-extractor/pkg/kube"
)

const yamlIndent = 2

// YAML writes runtime.Objects to the writer as a multi-document YAML stream.
// Accepts any slice type that satisfies runtime.Object constraint.
func YAML[T runtime.Object](w io.Writer, objects []T) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(yamlIndent)

	defer func() { _ = encoder.Close() }()

	for _, obj := range objects {
		// Convert to unstructured (fast path if already unstructured)
		u, err := kube.ToUnstructured(obj)
		if err != nil {
			return fmt.Errorf("failed to convert object: %w", err)
		}

		// Clean and encode
		cleaned := kube.CleanUnstructured(u)
		if err := encoder.Encode(cleaned.Object); err != nil {
			return fmt.Errorf("failed to encode object to YAML: %w", err)
		}
	}

	return nil
}
