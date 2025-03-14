package libs

import (
	"reflect"

	"github.com/google/cel-go/cel"
)

type Library interface {
	cel.Library

	NativeTypes() []reflect.Type
}
