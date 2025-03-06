package variables

import (
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	podImageExtractors = []v1alpha1.Image{
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
	}
	podControllerImageExtractors = []v1alpha1.Image{
		{
			Name:       "containers",
			Expression: "request.object.spec.template.spec.containers.map(e, e.image)",
		},
		{
			Name:       "initContainers",
			Expression: "request.object.spec.template.spec.initContainers.map(e, e.image)",
		},
		{
			Name:       "ephemeralContainers",
			Expression: "request.object.spec.template.spec.ephemeralContainers.map(e, e.image)",
		},
	}
	cronJobImageExtractors = []v1alpha1.Image{
		{
			Name:       "containers",
			Expression: "request.object.spec.jobTemplate.spec.template.spec.containers.map(e, e.image)",
		},
		{
			Name:       "initContainers",
			Expression: "request.object.spec.jobTemplate.spec.template.spec.initContainers.map(e, e.image)",
		},
		{
			Name:       "ephemeralContainers",
			Expression: "request.object.spec.jobTemplate.spec.template.spec.ephemeralContainers.map(e, e.image)",
		},
	}
)

type CompiledImageExtractor struct {
	key string
	e   cel.Program
}

func (c *CompiledImageExtractor) GetImages(data map[string]any) (string, []string, error) {
	out, _, err := c.e.Eval(data)
	if err != nil {
		return "", nil, err
	}

	result, err := utils.ConvertToNative[[]string](out)
	if err != nil {
		return "", nil, err
	}

	return c.key, result, nil
}

func CompileImageExtractors(path *field.Path, imageExtractors []v1alpha1.Image, gvr *metav1.GroupVersionResource, envOpts []cel.EnvOption) ([]*CompiledImageExtractor, field.ErrorList) {
	var allErrs field.ErrorList
	if gvr != nil {
		imageExtractors = append(imageExtractors, getExtractorForGVR(gvr)...)
	}

	compiledMatches := make([]*CompiledImageExtractor, 0, len(imageExtractors))
	e, err := cel.NewEnv(envOpts...)
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

func ExtractImages(c []*CompiledImageExtractor, data map[string]any) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, v := range c {
		if key, images, err := v.GetImages(data); err != nil {
			return nil, err
		} else {
			result[key] = images
		}
	}
	return result, nil
}

func getExtractorForGVR(gvr *metav1.GroupVersionResource) []v1alpha1.Image {
	if gvr == nil {
		return []v1alpha1.Image{}
	}

	if gvr.Group == "batch" && gvr.Version == "v1" {
		if gvr.Resource == "jobs" {
			return podControllerImageExtractors
		} else if gvr.Resource == "cronjobs" {
			return cronJobImageExtractors
		}
	}

	if gvr.Group == "apps" && gvr.Version == "v1" {
		r := gvr.Resource
		if r == "deployments" || r == "statefulsets" || r == "daemonsets" || r == "replicasets" {
			return podControllerImageExtractors
		}
	}

	if gvr.Group == "" && gvr.Version == "v1" && gvr.Resource == "pods" {
		return podImageExtractors
	}

	return []v1alpha1.Image{}
}
