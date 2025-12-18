package slices_test

import (
	"testing"

	"github.com/lburgazzoli/olm-extractor/pkg/util/slices"

	. "github.com/onsi/gomega"
)

func TestFind(t *testing.T) {
	t.Run("finds first match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3, 4, 5}
		result, found := slices.Find(numbers, func(n int) bool { return n > 2 })

		g.Expect(found).To(BeTrue())
		g.Expect(result).To(Equal(3))
	})

	t.Run("returns false when not found", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3}
		result, found := slices.Find(numbers, func(n int) bool { return n > 10 })

		g.Expect(found).To(BeFalse())
		g.Expect(result).To(Equal(0))
	})

	t.Run("handles empty slice", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{}
		result, found := slices.Find(numbers, func(n int) bool { return n > 0 })

		g.Expect(found).To(BeFalse())
		g.Expect(result).To(Equal(0))
	})

	t.Run("works with strings", func(t *testing.T) {
		g := NewWithT(t)
		words := []string{"hello", "world", "test"}
		result, found := slices.Find(words, func(s string) bool { return s == "world" })

		g.Expect(found).To(BeTrue())
		g.Expect(result).To(Equal("world"))
	})
}

func TestFilter(t *testing.T) {
	t.Run("filters matching elements", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3, 4, 5}
		result := slices.Filter(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(Equal([]int{2, 4}))
	})

	t.Run("returns empty slice when no matches", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 3, 5}
		result := slices.Filter(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(BeEmpty())
	})

	t.Run("handles empty slice", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{}
		result := slices.Filter(numbers, func(n int) bool { return n > 0 })

		g.Expect(result).To(BeEmpty())
	})

	t.Run("returns all when all match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{2, 4, 6}
		result := slices.Filter(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(Equal(numbers))
	})
}

func TestMap(t *testing.T) {
	t.Run("transforms elements", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3}
		result := slices.Map(numbers, func(n int) int { return n * 2 })

		g.Expect(result).To(Equal([]int{2, 4, 6}))
	})

	t.Run("handles empty slice", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{}
		result := slices.Map(numbers, func(n int) int { return n * 2 })

		g.Expect(result).To(BeEmpty())
	})

	t.Run("changes type", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3}
		result := slices.Map(numbers, func(n int) string { return string(rune('a' + n - 1)) })

		g.Expect(result).To(Equal([]string{"a", "b", "c"}))
	})

	t.Run("extracts struct field", func(t *testing.T) {
		g := NewWithT(t)
		type Person struct {
			Name string
			Age  int
		}
		people := []Person{
			{Name: "Alice", Age: 30},
			{Name: "Bob", Age: 25},
		}
		result := slices.Map(people, func(p Person) string { return p.Name })

		g.Expect(result).To(Equal([]string{"Alice", "Bob"}))
	})
}

func TestAny(t *testing.T) {
	t.Run("returns true when any match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3, 4, 5}
		result := slices.Any(numbers, func(n int) bool { return n > 3 })

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when none match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3}
		result := slices.Any(numbers, func(n int) bool { return n > 10 })

		g.Expect(result).To(BeFalse())
	})

	t.Run("handles empty slice", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{}
		result := slices.Any(numbers, func(n int) bool { return n > 0 })

		g.Expect(result).To(BeFalse())
	})

	t.Run("returns true for first match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3, 4, 5}
		result := slices.Any(numbers, func(n int) bool { return n == 1 })

		g.Expect(result).To(BeTrue())
	})
}

func TestAll(t *testing.T) {
	t.Run("returns true when all match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{2, 4, 6, 8}
		result := slices.All(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false when one does not match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{2, 3, 4}
		result := slices.All(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(BeFalse())
	})

	t.Run("handles empty slice", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{}
		result := slices.All(numbers, func(n int) bool { return n > 0 })

		g.Expect(result).To(BeTrue())
	})

	t.Run("returns false on first non-match", func(t *testing.T) {
		g := NewWithT(t)
		numbers := []int{1, 2, 3, 4}
		result := slices.All(numbers, func(n int) bool { return n%2 == 0 })

		g.Expect(result).To(BeFalse())
	})
}
