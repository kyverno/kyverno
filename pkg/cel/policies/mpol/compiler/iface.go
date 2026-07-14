package compiler

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

type Patcher interface {
	Patch(context.Context, map[string]any, patch.Request, int64) (runtime.Object, error)
}
