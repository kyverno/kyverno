package unstructured

import (
	"strings"

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

func FixupGenerateLabels(obj unstructured.Unstructured) {
	tidy := map[string]string{
		"app.kubernetes.io/managed-by": "kyverno",
	}
	if labels := obj.GetLabels(); labels != nil {
		for k, v := range labels {
			if !strings.HasPrefix(k, "generate.kyverno.io/") {
				tidy[k] = v
			}
		}
	}
	obj.SetLabels(tidy)
}

func Compare(a, e unstructured.Unstructured, tidy bool) (bool, error) {
	if tidy {
		a = Tidy(a)
		e = Tidy(e)
	}
	actual, err := a.MarshalJSON()
	if err != nil {
		return false, err
	}
	expected, err := e.MarshalJSON()
	if err != nil {
		return false, err
	}
	patch, err := jsonpatch.CreateMergePatch(actual, expected)
	if err != nil {
		return false, err
	}
	return len(patch) == 2, nil
}
