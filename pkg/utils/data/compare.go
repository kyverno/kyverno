package data

import (
	"golang.org/x/exp/constraints"
)

func Compare[T constraints.Ordered](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
