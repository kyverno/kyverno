package jmespath

import "fmt"

// NotFoundError is returned when it is impossible to resolve the AstField.
type NotFoundError struct {
	key string
}

func (n NotFoundError) Error() string {
	return fmt.Sprintf("Unknown key \"%s\" in path", n.key)
}

func NotFound(key string) NotFoundError {
	return NotFoundError{key}
}
