package data

import "reflect"

func DeepEqual[T any](a T, b T) bool {
	return reflect.DeepEqual(a, b)
}
