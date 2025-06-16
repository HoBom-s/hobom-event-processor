package utils

// `ForEach` executes the action for each element in the parameter.
func ForEach[T any](arr []T, action func(T)) {
	for _, v := range arr {
		action(v)
	}
}

// `Map` applies the mapper function to each element in the parameter
// and returns a new array with the results.
func Map[T any, R any](arr []T, action func(T) R) []R {
	result := make([]R, 0, len(arr))
	for _, v := range arr {
		result = append(result, action(v))
	}

	return result
}

// Filter returns a new array containing only the elements
// for which the predicate returns true.
func Filter[T any](arr []T, pred func(T) bool) []T {
	var result []T
	for _, v := range arr {
		if pred(v) {
			result = append(result, v)
		}
	}

	return result
}