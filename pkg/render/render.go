package render

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/runtime"
)

const yamlIndent = 2

// YAML writes the given runtime.Objects to the writer as a multi-document YAML stream.
func YAML(w io.Writer, objects []runtime.Object) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(yamlIndent)

	defer func() { _ = encoder.Close() }()

	for _, obj := range objects {
		cleaned, err := ToUnstructured(obj)
		if err != nil {
			return err
		}

		if err := encoder.Encode(cleaned); err != nil {
			return fmt.Errorf("failed to encode object to YAML: %w", err)
		}
	}

	return nil
}

// ToUnstructured converts a runtime.Object to an unstructured map and removes nil/empty fields.
func ToUnstructured(obj runtime.Object) (map[string]any, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
	}

	return CleanUnstructured(unstructuredMap), nil
}

// CleanUnstructured recursively removes nil values and empty maps/slices from the object.
func CleanUnstructured(obj map[string]any) map[string]any {
	result := make(map[string]any)

	for key, value := range obj {
		cleaned := cleanValue(value)
		if cleaned != nil {
			result[key] = cleaned
		}
	}

	return result
}

func cleanValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case map[string]any:
		cleaned := CleanUnstructured(v)
		if len(cleaned) == 0 {
			return nil
		}

		return cleaned
	case []any:
		if len(v) == 0 {
			return nil
		}

		cleaned := make([]any, 0, len(v))

		for _, item := range v {
			if c := cleanValue(item); c != nil {
				cleaned = append(cleaned, c)
			}
		}

		if len(cleaned) == 0 {
			return nil
		}

		return cleaned
	case string:
		if v == "" {
			return nil
		}

		return v
	default:
		return value
	}
}
