package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Tidy(obj unstructured.Unstructured) unstructured.Unstructured {
	if obj.Object == nil {
		return obj
	}
	return unstructured.Unstructured{
		Object: tidy(obj.UnstructuredContent()).(map[string]interface{}),
	}
}

func tidy(obj interface{}) interface{} {
	switch typedPatternElement := obj.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, v := range typedPatternElement {
			v = tidy(v)
			if v != nil {
				out[k] = v
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []interface{}:
		var out []interface{}
		for _, v := range typedPatternElement {
			v = tidy(v)
			if v != nil {
				out = append(out, v)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return obj
	}
}
