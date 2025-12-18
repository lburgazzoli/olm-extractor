package slices

// Find returns the first element matching the predicate.
// Returns zero value and false if not found.
func Find[T any](slice []T, predicate func(T) bool) (T, bool) {
	for _, item := range slice {
		if predicate(item) {
			return item, true
		}
	}

	var zero T

	return zero, false
}

// Filter returns all elements matching the predicate.
func Filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if predicate(item) {
			result = append(result, item)
		}
	}

	return result
}

// Map transforms each element using the mapper function.
func Map[T, U any](slice []T, mapper func(T) U) []U {
	result := make([]U, len(slice))
	for i, item := range slice {
		result[i] = mapper(item)
	}

	return result
}

// Any returns true if any element matches the predicate.
func Any[T any](slice []T, predicate func(T) bool) bool {
	for _, item := range slice { //nolint:modernize
		if predicate(item) {
			return true
		}
	}

	return false
}

// All returns true if all elements match the predicate.
func All[T any](slice []T, predicate func(T) bool) bool {
	for _, item := range slice {
		if !predicate(item) {
			return false
		}
	}

	return true
}
