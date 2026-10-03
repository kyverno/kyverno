package compiler

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
)

// MutationEval is the raw CEL output of a mutation expression's evaluation, surfaced purely for
// tracing: the same Result/Details a validation's ContextEval call already returns, kept
// alongside the patched object rather than replacing it. Result is always set; Details is nil
// when the patcher's Program wasn't built with tracing on, same as any other ContextEval call --
// trace.Build already tolerates a nil *cel.EvalDetails.
type MutationEval struct {
	Result  ref.Val
	Details *cel.EvalDetails
}

type Patcher interface {
	Patch(context.Context, map[string]any, patch.Request, int64) (runtime.Object, *MutationEval, error)
}
