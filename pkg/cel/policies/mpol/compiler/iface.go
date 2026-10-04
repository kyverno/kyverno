package compiler

import (
	"context"

	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

// MutationEval is the raw CEL result of a mutation expression's evaluation, surfaced purely for
// tracing and kept alongside the patched object rather than replacing it. The per-node values come
// from re-running the expression's tracking twin (see compiler.TracedProgram), not from here.
type MutationEval struct {
	Result ref.Val
}

type Patcher interface {
	Patch(context.Context, map[string]any, patch.Request, int64) (runtime.Object, *MutationEval, error)
}
