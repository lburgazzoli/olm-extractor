package filter

import (
	"fmt"

	"github.com/itchyny/gojq"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Filter applies jq expressions to filter Kubernetes resources.
// Exclude filters take priority over include filters.
type Filter struct {
	includeQueries []*gojq.Query
	excludeQueries []*gojq.Query
}

// New creates a new Filter with compiled jq expressions.
// Returns an error if any expression fails to compile.
func New(includeExprs []string, excludeExprs []string) (*Filter, error) {
	f := &Filter{
		includeQueries: make([]*gojq.Query, 0, len(includeExprs)),
		excludeQueries: make([]*gojq.Query, 0, len(excludeExprs)),
	}

	for _, expr := range includeExprs {
		query, err := gojq.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid include expression %q: %w", expr, err)
		}
		f.includeQueries = append(f.includeQueries, query)
	}

	for _, expr := range excludeExprs {
		query, err := gojq.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude expression %q: %w", expr, err)
		}
		f.excludeQueries = append(f.excludeQueries, query)
	}

	return f, nil
}

// Matches returns true if the object should be included based on filter rules.
// Exclude filters have priority: if a resource matches any exclude filter, it returns false.
// Include filters act as OR: if include filters exist, the resource must match at least one.
func (f *Filter) Matches(obj *unstructured.Unstructured) bool {
	return f.shouldInclude(obj.Object)
}

// shouldInclude determines if an object should be included based on filter rules.
func (f *Filter) shouldInclude(objMap map[string]any) bool {
	// Check exclude filters first (they have priority)
	for _, query := range f.excludeQueries {
		if matchesQuery(query, objMap) {
			return false
		}
	}

	// If no include filters, include by default (already passed exclude check)
	if len(f.includeQueries) == 0 {
		return true
	}

	// Check if matches any include filter (OR logic)
	for _, query := range f.includeQueries {
		if matchesQuery(query, objMap) {
			return true
		}
	}

	return false
}

// matchesQuery evaluates a jq query against an object and returns true if it matches.
// A match is defined as the query returning the boolean value true.
func matchesQuery(query *gojq.Query, obj map[string]any) bool {
	iter := query.Run(obj)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		// If there's an error, treat as no match
		if _, isErr := v.(error); isErr {
			return false
		}

		// Only match if the result is the boolean value true
		if b, ok := v.(bool); ok && b {
			return true
		}
	}

	return false
}
