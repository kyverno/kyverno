package match

import (
	"fmt"
	"log"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type CompiledMatch struct {
	g glob.Glob
	e cel.Program
}

func (c *CompiledMatch) Match(image string) (bool, error) {
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

func Match(c []*CompiledMatch, image string) (bool, error) {
	if len(c) == 0 {
		return true, nil
	}
	for _, v := range c {
		if matched, err := v.Match(image); err != nil {
			return false, err
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}

func CompileMatches(path *field.Path, matches []v1alpha1.ImageRule) ([]*CompiledMatch, field.ErrorList) {
	var allErrs field.ErrorList
	compiledMatches := make([]*CompiledMatch, 0, len(matches))
	e, err := cel.NewEnv(
		cel.Variable("ref", cel.StringType),
	)
	if err != nil {
		return nil, append(allErrs, field.Invalid(path, matches, err.Error()))
	}

	for i, m := range matches {
		c := &CompiledMatch{}
		if m.Glob != "" {
			path := path.Index(i).Child("glob")
			g, err := glob.Compile(m.Glob)
			if err != nil {
				return nil, append(allErrs, field.Invalid(path, m.Glob, err.Error()))
			}
			c.g = g
		} else if m.CELExpression != "" {
			path := path.Index(i).Child("expression")
			ast, iss := e.Compile(m.CELExpression)
			if iss.Err() != nil {
				return nil, append(allErrs, field.Invalid(path, m.CELExpression, iss.Err().Error()))
			}
			prg, err := e.Program(ast)
			if err != nil {
				return nil, append(allErrs, field.Invalid(path, m.CELExpression, err.Error()))
			}
			c.e = prg
		}
		compiledMatches = append(compiledMatches, c)
	}

	return compiledMatches, nil
}
