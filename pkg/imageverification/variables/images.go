package variables

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var podImageExtractors = []v1alpha1.Image{
	{
		Name:       "containers",
		Expression: "request.object.spec.containers.map(e, e.image)",
	},
	{
		Name:       "initContainers",
		Expression: "request.object.spec.initContainers.map(e, e.image)",
	},
	{
		Name:       "ephemeralContainers",
		Expression: "request.object.spec.ephemeralContainers.map(e, e.image)",
	},
	// TODO: add one for all
}

type CompiledImageExtractor struct {
	key string
	e   cel.Program
}

func (c *CompiledImageExtractor) GetImages(request interface{}) (string, []string, error) {
	out, _, err := c.e.Eval(map[string]any{
		policy.RequestKey: request,
	})
	if err != nil {
		return "", nil, err
	}

	result, err := utils.ConvertToNative[[]string](out)
	if err != nil {
		return "", nil, err
	}

	return c.key, result, nil
}

func CompileImageExtractors(path *field.Path, imageExtractors []v1alpha1.Image, isPod bool) ([]*CompiledImageExtractor, field.ErrorList) {
	var allErrs field.ErrorList
	if isPod {
		imageExtractors = append(imageExtractors, podImageExtractors...)
	}

	compiledMatches := make([]*CompiledImageExtractor, 0, len(imageExtractors))
	e, err := cel.NewEnv(
		// this uses dyn type to allow unstructured data
		cel.Variable(policy.RequestKey, types.DynType),
	)
	if err != nil {
		return nil, append(allErrs, field.Invalid(path, imageExtractors, err.Error()))
	}

	for i, m := range imageExtractors {
		path := path.Index(i).Child("expression")
		c := &CompiledImageExtractor{
			key: m.Name,
		}
		ast, iss := e.Compile(m.Expression)
		if iss.Err() != nil {
			return nil, append(allErrs, field.Invalid(path, m.Expression, iss.Err().Error()))
		}
		prg, err := e.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, m.Expression, err.Error()))
		}
		c.e = prg
		compiledMatches = append(compiledMatches, c)
	}

	return compiledMatches, nil
}

func ExtractImages(c []*CompiledImageExtractor, request interface{}) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, v := range c {
		if key, images, err := v.GetImages(request); err != nil {
			return nil, err
		} else {
			result[key] = images
		}
	}
	return result, nil
}
