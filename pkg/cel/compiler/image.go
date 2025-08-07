package compiler

import (
	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type MatchImageReference interface {
	Match(string) (bool, error)
}

type matchGlob struct {
	glob.Glob
}

func (m *matchGlob) Match(image string) (bool, error) {
	return m.Glob.Match(image), nil
}

type matchCel struct {
	cel.Program
}

func (m *matchCel) Match(image string) (bool, error) {
	out, _, err := m.Program.Eval(map[string]any{
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
