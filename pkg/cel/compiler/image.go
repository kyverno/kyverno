package compiler

import (
	"context"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/kyverno/sdk/extensions/cel/utils"
)

// Match takes a context so the CEL implementation can charge the request cost budget.
// A bare Program.Eval cannot, and an ImageValidatingPolicy may declare any number of
// matchImageReferences (the CRD sets no maxItems), so without it each expression is
// capped individually while their total goes unaccounted.
type MatchImageReference interface {
	Match(context.Context, string) (bool, error)
}

type matchGlob struct {
	glob.Glob
}

func (m *matchGlob) Match(_ context.Context, image string) (bool, error) {
	return m.Glob.Match(image), nil
}

type matchCel struct {
	cel.Program
}

func (m *matchCel) Match(ctx context.Context, image string) (bool, error) {
	out, _, err := m.Program.ContextEval(ctx, map[string]any{
		ImageRefKey: image,
	})
	if err != nil {
		return false, err
	}
	result, err := utils.ConvertToNative[bool](out)
	if err != nil {
		return false, err
	}
	return result, nil
}
