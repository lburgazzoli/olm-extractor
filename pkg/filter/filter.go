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
	includeQueries, err := parseAll(includeExprs, "include")
	if err != nil {
		return nil, err
	}

	excludeQueries, err := parseAll(excludeExprs, "exclude")
	if err != nil {
		return nil, err
	}

	return &Filter{
		includeQueries: includeQueries,
		excludeQueries: excludeQueries,
	}, nil
}

// Matches returns true if the object should be included based on filter rules.
// Exclude filters have priority: if a resource matches any exclude filter, it returns false.
// Include filters act as OR: if include filters exist, the resource must match at least one.
// Returns an error if any jq query execution fails.
func (f *Filter) Matches(obj *unstructured.Unstructured) (bool, error) {
	return f.shouldInclude(obj.Object)
}

// shouldInclude determines if an object should be included based on filter rules.
func (f *Filter) shouldInclude(objMap map[string]any) (bool, error) {
	// Check exclude filters first (ANY match = exclude)
	excluded, err := matchAny(f.excludeQueries, objMap)
	if err != nil {
		return false, fmt.Errorf("exclude filter error: %w", err)
	}
	if excluded {
		return false, nil
	}

	// If no include filters, include by default (already passed exclude check)
	if len(f.includeQueries) == 0 {
		return true, nil
	}

	// Check if matches any include filter (OR logic)
	included, err := matchAny(f.includeQueries, objMap)
	if err != nil {
		return false, fmt.Errorf("include filter error: %w", err)
	}

	return included, nil
}

// parseAll compiles multiple jq expressions into queries.
// Returns an error if any expression fails to compile.
func parseAll(exprs []string, filterType string) ([]*gojq.Query, error) {
	queries := make([]*gojq.Query, 0, len(exprs))

	for _, expr := range exprs {
		query, err := gojq.Parse(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s expression %q: %w", filterType, expr, err)
		}
		queries = append(queries, query)
	}

	return queries, nil
}

// matchAny returns true if any of the queries match the object.
// Returns an error if any query execution fails.
func matchAny(queries []*gojq.Query, obj map[string]any) (bool, error) {
	for _, query := range queries {
		matches, err := matchesQuery(query, obj)
		if err != nil {
			return false, err
		}
		if matches {
			return true, nil
		}
	}

	return false, nil
}

// matchesQuery evaluates a jq query against an object and returns true if it matches.
// A match is defined as the query returning the boolean value true.
// Returns an error if the jq query execution fails.
func matchesQuery(query *gojq.Query, obj map[string]any) (bool, error) {
	iter := query.Run(obj)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		// Check for query execution errors
		if err, isErr := v.(error); isErr {
			return false, fmt.Errorf("jq query execution failed: %w", err)
		}

		// Only match if the result is the boolean value true
		if b, ok := v.(bool); ok && b {
			return true, nil
		}
	}

	return false, nil
}
