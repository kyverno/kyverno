package unstructured

import (
	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TidyObject(obj interface{}) interface{} {
	switch typedPatternElement := obj.(type) {
	case map[string]interface{}:
		tidy := map[string]interface{}{}
		for k, v := range typedPatternElement {
			v = TidyObject(v)
			if v != nil {
				tidy[k] = v
			}
		}
		if len(tidy) == 0 {
			return nil
		}
		return tidy
	case []interface{}:
		var tidy []interface{}
		for _, v := range typedPatternElement {
			v = TidyObject(v)
			if v != nil {
				tidy = append(tidy, v)
			}
		}
		if len(tidy) == 0 {
			return nil
		}
		return tidy
	default:
		return obj
	}
}

func Tidy(obj unstructured.Unstructured) unstructured.Unstructured {
	if obj.Object == nil {
		return obj
	}
	return unstructured.Unstructured{
		Object: TidyObject(obj.UnstructuredContent()).(map[string]interface{}),
	}
}

func Compare(a, b unstructured.Unstructured, tidy bool) (bool, error) {
	if tidy {
		a = Tidy(a)
		b = Tidy(b)
	}
	expected, err := a.MarshalJSON()
	if err != nil {
		return false, err
	}
	actual, err := b.MarshalJSON()
	if err != nil {
		return false, err
	}
	patch, err := jsonpatch.CreateMergePatch(actual, expected)
	if err != nil {
		return false, err
	}
	return len(patch) == 2, nil
}
