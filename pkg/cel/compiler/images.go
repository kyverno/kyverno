package compiler

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
		Expression: "(object != null ? object : oldObject).spec.?containers.orValue([]).map(e, e.image)",
	}, {
		Name:       "initContainers",
		Expression: "(object != null ? object : oldObject).spec.?initContainers.orValue([]).map(e, e.image)",
	}, {
		Name:       "ephemeralContainers",
		Expression: "(object != null ? object : oldObject).spec.?ephemeralContainers.orValue([]).map(e, e.image)",
	}}
	podControllerImageExtractors = []v1alpha1.ImageExtractor{{
		Name:       "containers",
		Expression: "(object != null ? object : oldObject).spec.template.spec.?containers.orValue([]).map(e, e.image)",
	}, {
		Name:       "initContainers",
		Expression: "(object != null ? object : oldObject).spec.template.spec.?initContainers.orValue([]).map(e, e.image)",
	}, {
		Name:       "ephemeralContainers",
		Expression: "(object != null ? object : oldObject).spec.template.spec.?ephemeralContainers.orValue([]).map(e, e.image)",
	}}
	cronJobImageExtractors = []v1alpha1.ImageExtractor{{
		Name:       "containers",
		Expression: "(object != null ? object : oldObject).spec.jobTemplate.spec.template.spec.?containers.orValue([]).map(e, e.image)",
	}, {
		Name:       "initContainers",
		Expression: "(object != null ? object : oldObject).spec.jobTemplate.spec.template.spec.?initContainers.orValue([]).map(e, e.image)",
	}, {
		Name:       "ephemeralContainers",
		Expression: "(object != null ? object : oldObject).spec.jobTemplate.spec.template.spec.?ephemeralContainers.orValue([]).map(e, e.image)",
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

type ImageExtractor struct {
	cel.Program
}

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

func (c *ImageExtractor) GetImages(data map[string]any) ([]string, error) {
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

func CompileImageExtractors(path *field.Path, env *cel.Env, gvr *metav1.GroupVersionResource, imageExtractors ...v1alpha1.ImageExtractor) (map[string]ImageExtractor, field.ErrorList) {
	var extractors []v1alpha1.ImageExtractor
	if gvr != nil {
		extractors = append(extractors, getImageExtractorsFromGVR(*gvr)...)
	}
	extractors = append(extractors, imageExtractors...)
	if len(extractors) == 0 {
		return nil, nil
	}
	var allErrs field.ErrorList
	compiled := make(map[string]ImageExtractor, len(extractors))
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
		compiled[m.Name] = ImageExtractor{
			Program: prog,
		}
	}
	return compiled, nil
}

func ExtractImages(data map[string]any, extractors map[string]ImageExtractor) (map[string][]string, error) {
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
