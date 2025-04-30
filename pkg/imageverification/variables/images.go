package variables

import (
	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	podImageExtractors = []v1alpha1.ImageExtractor{{
		Name:       "containers",
		Expression: "has(object.spec.containers) ? object.spec.containers.map(e, e.image) : []",
	}, {
		Name:       "initContainers",
		Expression: "has(object.spec.initContainers) ? object.spec.initContainers.map(e, e.image) : []",
	}, {
		Name:       "ephemeralContainers",
		Expression: "has(object.spec.ephemeralContainers) ? object.spec.ephemeralContainers.map(e, e.image) : []",
	}}
	podControllerImageExtractors = []v1alpha1.ImageExtractor{{
		Name:       "containers",
		Expression: "has(object.spec.template.spec.containers) ? object.spec.template.spec.containers.map(e, e.image) : []",
	}, {
		Name:       "initContainers",
		Expression: "has(object.spec.template.spec.initContainers) ? object.spec.template.spec.initContainers.map(e, e.image) : []",
	}, {
		Name:       "ephemeralContainers",
		Expression: "has(object.spec.template.spec.ephemeralContainers) ? object.spec.template.spec.ephemeralContainers.map(e, e.image) : []",
	}}
	cronJobImageExtractors = []v1alpha1.ImageExtractor{{
		Name:       "containers",
		Expression: "has(object.spec.jobTemplate.spec.template.spec.containers) ? object.spec.jobTemplate.spec.template.spec.containers.map(e, e.image) : []",
	}, {
		Name:       "initContainers",
		Expression: "has(object.spec.jobTemplate.spec.template.spec.initContainers) ? object.spec.jobTemplate.spec.template.spec.initContainers.map(e, e.image) : []",
	}, {
		Name:       "ephemeralContainers",
		Expression: "has(object.spec.jobTemplate.spec.template.spec.ephemeralContainers) ? object.spec.jobTemplate.spec.template.spec.ephemeralContainers.map(e, e.image) : []",
	}}
)

var (
	pods         = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	jobs         = metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	deployments  = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	statefulsets = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	daemonsets   = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	replicasets  = metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	cronjobs     = metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
)

// TODO: return compiled version directly
func getImageExtractorsFromGVR(gvr metav1.GroupVersionResource) []v1alpha1.ImageExtractor {
	switch gvr {
	case pods:
		return podImageExtractors
	case jobs, deployments, statefulsets, daemonsets, replicasets:
		return podControllerImageExtractors
	case cronjobs:
		return cronJobImageExtractors
	}
	return nil
}

type CompiledImageExtractor struct {
	cel.Program
}

func (c *CompiledImageExtractor) GetImages(data map[string]any) ([]string, error) {
	out, _, err := c.Eval(data)
	if err != nil {
		return nil, err
	}
	result, err := utils.ConvertToNative[[]string](out)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func CompileImageExtractors(path *field.Path, envOpts []cel.EnvOption, gvr *metav1.GroupVersionResource, imageExtractors ...v1alpha1.ImageExtractor) (map[string]CompiledImageExtractor, field.ErrorList) {
	var extractors []v1alpha1.ImageExtractor
	if gvr != nil {
		extractors = append(extractors, getImageExtractorsFromGVR(*gvr)...)
	}
	extractors = append(extractors, imageExtractors...)
	if len(extractors) == 0 {
		return nil, nil
	}
	var allErrs field.ErrorList
	compiled := make(map[string]CompiledImageExtractor, len(extractors))
	env, err := cel.NewEnv(envOpts...)
	if err != nil {
		return nil, append(allErrs, field.InternalError(path, err))
	}
	for i, m := range extractors {
		path := path.Index(i).Child("expression")
		ast, iss := env.Compile(m.Expression)
		if iss.Err() != nil {
			return nil, append(allErrs, field.Invalid(path, m.Expression, iss.Err().Error()))
		}
		// TODO: check output type
		prog, err := env.Program(ast)
		if err != nil {
			return nil, append(allErrs, field.Invalid(path, m.Expression, err.Error()))
		}
		compiled[m.Name] = CompiledImageExtractor{
			Program: prog,
		}
	}
	return compiled, nil
}

func ExtractImages(data map[string]any, extractors map[string]CompiledImageExtractor) (map[string][]string, error) {
	result := make(map[string][]string, len(extractors))
	for key, value := range extractors {
		if images, err := value.GetImages(data); err != nil {
			return nil, err
		} else {
			result[key] = images
		}
	}
	return result, nil
}
