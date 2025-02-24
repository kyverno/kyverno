package match

import (
	"fmt"
	"log"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type compiledMatch struct {
	g glob.Glob
	e cel.Program
}

func (c *compiledMatch) Match(image string) (bool, error) {
	if c.g != nil {
		return c.g.Match(image), nil
	} else if c.e != nil {
		out, _, err := c.e.Eval(map[string]any{
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
	} else {
		return false, fmt.Errorf("invalid match block")
	}
}

func Match(c []compiledMatch, image string) (bool, error) {
	for _, v := range c {
		if matched, err := v.Match(image); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}

func CompiledMatches(matches []v1alpha1.ImageRule) ([]compiledMatch, error) {
	compiledMatches := make([]compiledMatch, 0, len(matches))
	e, err := cel.NewEnv(
		cel.Variable("ref", cel.StringType),
	)
	if err != nil {
		return nil, err
	}

	for _, m := range matches {
		var c compiledMatch
		if m.Glob != "" {
			g, err := glob.Compile(m.Glob)
			if err != nil {
				return nil, err
			}
			c.g = g
		} else if m.CELExpression != "" {
			ast, iss := e.Compile(m.CELExpression)
			if iss.Err() != nil {
				return nil, iss.Err()
			}
			prg, err := e.Program(ast)
			if err != nil {
				return nil, err
			}
			c.e = prg
		}
		compiledMatches = append(compiledMatches, c)
	}

	return compiledMatches, nil
}
