package main

import (
	"testing"

	. "github.com/onsi/gomega"
)

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
			g.Expect(validateNamespace(name)).To(Succeed(), "expected %q to be valid", name)
		}
	})

	t.Run("rejects empty namespace", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("")).To(MatchError("namespace cannot be empty"))
	})

	t.Run("rejects namespace longer than 63 characters", func(t *testing.T) {
		g := NewWithT(t)

		longName := "a123456789012345678901234567890123456789012345678901234567890123" // 64 chars
		g.Expect(validateNamespace(longName)).To(MatchError("namespace name too long (max 63 characters)"))
	})

	t.Run("rejects namespace starting with digit", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("1test")).To(MatchError("invalid namespace name: must start with a lowercase letter"))
	})

	t.Run("rejects namespace starting with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("-test")).To(MatchError("invalid namespace name: must start with a lowercase letter"))
	})

	t.Run("rejects namespace ending with dash", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("test-")).To(MatchError("invalid namespace name: must end with an alphanumeric character"))
	})

	t.Run("rejects namespace with uppercase letters", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("Test")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})

	t.Run("rejects namespace with underscores", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("test_ns")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})

	t.Run("rejects namespace with dots", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(validateNamespace("test.ns")).To(MatchError("invalid namespace name: must consist of lowercase alphanumeric characters or '-'"))
	})
}
