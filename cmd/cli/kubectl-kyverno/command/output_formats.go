package command

import "strings"

// OutputFormats pairs each flag value with its renderer in completion order.
type OutputFormats[T any] []struct {
	Name   string
	Render T
}

func (formats OutputFormats[T]) Names(prefix string) []string {
	var names []string
	for _, format := range formats {
		if strings.HasPrefix(format.Name, prefix) {
			names = append(names, format.Name)
		}
	}
	return names
}

func (formats OutputFormats[T]) Lookup(name string) (T, bool) {
	for _, format := range formats {
		if format.Name == name {
			return format.Render, true
		}
	}
	var zero T
	return zero, false
}
