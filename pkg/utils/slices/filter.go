package slices

func Filter[T any](list []T, filter func(T) bool) []T {
	filtered := make([]T, 0, len(list))
	for _, item := range list {
		if filter(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
