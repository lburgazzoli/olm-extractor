package render_test

import (
	"testing"

	"github.com/lburgazzoli/olm-extractor/pkg/render"

	. "github.com/onsi/gomega"
)

func TestCleanUnstructured(t *testing.T) {
	g := NewWithT(t)

	t.Run("removes nil values", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"name":      "test",
			"namespace": nil,
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKey("name"))
		g.Expect(result).NotTo(HaveKey("namespace"))
	})

	t.Run("removes empty maps", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"metadata": map[string]any{
				"name": "test",
			},
			"status": map[string]any{},
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKey("metadata"))
		g.Expect(result).NotTo(HaveKey("status"))
	})

	t.Run("removes empty slices", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"containers":     []any{map[string]any{"name": "app"}},
			"emptyList":      []any{},
			"initContainers": []any{},
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKey("containers"))
		g.Expect(result).NotTo(HaveKey("emptyList"))
		g.Expect(result).NotTo(HaveKey("initContainers"))
	})

	t.Run("removes empty strings", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"name":        "test",
			"emptyString": "",
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKey("name"))
		g.Expect(result).NotTo(HaveKey("emptyString"))
	})

	t.Run("preserves non-empty values", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"name":     "test",
			"replicas": int64(3),
			"enabled":  true,
			"disabled": false,
			"zero":     int64(0),
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKeyWithValue("name", "test"))
		g.Expect(result).To(HaveKeyWithValue("replicas", int64(3)))
		g.Expect(result).To(HaveKeyWithValue("enabled", true))
		g.Expect(result).To(HaveKeyWithValue("disabled", false))
		g.Expect(result).To(HaveKeyWithValue("zero", int64(0)))
	})

	t.Run("handles nested structures", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"metadata": map[string]any{
				"name":              "test",
				"namespace":         "default",
				"creationTimestamp": nil,
				"labels":            map[string]any{},
			},
			"spec": map[string]any{
				"replicas": int64(1),
				"selector": map[string]any{
					"matchLabels": map[string]any{
						"app": "test",
					},
				},
			},
			"status": map[string]any{},
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKey("metadata"))
		g.Expect(result).To(HaveKey("spec"))
		g.Expect(result).NotTo(HaveKey("status"))

		metadata := result["metadata"].(map[string]any)
		g.Expect(metadata).To(HaveKeyWithValue("name", "test"))
		g.Expect(metadata).To(HaveKeyWithValue("namespace", "default"))
		g.Expect(metadata).NotTo(HaveKey("creationTimestamp"))
		g.Expect(metadata).NotTo(HaveKey("labels"))

		spec := result["spec"].(map[string]any)
		g.Expect(spec).To(HaveKey("selector"))

		selector := spec["selector"].(map[string]any)
		g.Expect(selector).To(HaveKey("matchLabels"))
	})

	t.Run("cleans nested slices", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"containers": []any{
				map[string]any{
					"name":      "app",
					"image":     "nginx",
					"resources": map[string]any{},
				},
				map[string]any{
					"name":  "sidecar",
					"image": "proxy",
				},
			},
		}

		result := render.CleanUnstructured(input)

		containers := result["containers"].([]any)
		g.Expect(containers).To(HaveLen(2))

		container1 := containers[0].(map[string]any)
		g.Expect(container1).To(HaveKeyWithValue("name", "app"))
		g.Expect(container1).To(HaveKeyWithValue("image", "nginx"))
		g.Expect(container1).NotTo(HaveKey("resources"))
	})

	t.Run("returns empty map for all-nil input", func(t *testing.T) {
		input := map[string]any{
			"field1": nil,
			"field2": map[string]any{},
			"field3": []any{},
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(BeEmpty())
	})

	t.Run("preserves integers including zero", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"count": int64(42),
			"zero":  int64(0),
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKeyWithValue("count", int64(42)))
		g.Expect(result).To(HaveKeyWithValue("zero", int64(0)))
	})

	t.Run("preserves booleans including false", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"enabled":  true,
			"disabled": false,
		}

		result := render.CleanUnstructured(input)

		g.Expect(result["enabled"]).To(BeTrue())
		g.Expect(result["disabled"]).To(BeFalse())
	})

	t.Run("preserves floats including zero", func(t *testing.T) {
		g := NewWithT(t)

		input := map[string]any{
			"pi":   float64(3.14),
			"zero": float64(0),
		}

		result := render.CleanUnstructured(input)

		g.Expect(result).To(HaveKeyWithValue("pi", float64(3.14)))
		g.Expect(result).To(HaveKeyWithValue("zero", float64(0)))
	})
}
