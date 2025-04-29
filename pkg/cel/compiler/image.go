package compiler

import (
	"log"

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
		"ref": image,
	})
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}
	result, err := utils.ConvertToNative[bool](out)
	if err != nil {
		return false, err
	}
	return result, nil
}
