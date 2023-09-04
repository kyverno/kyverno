package unstructured

import (
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type (
	marshaler = func(*unstructured.Unstructured) ([]byte, error)
	patcher   = func(originalJSON, modifiedJSON []byte) ([]byte, error)
)

var (
	defaultMarshaler = (*unstructured.Unstructured).MarshalJSON
	defaultPatcher   = jsonpatch.CreateMergePatch
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
	return compare(a, e, defaultMarshaler, defaultPatcher)
}

func compare(a, e unstructured.Unstructured, marshaler marshaler, patcher patcher) (bool, error) {
	if marshaler == nil {
		marshaler = defaultMarshaler
	}
	actual, err := marshaler(&a)
	if err != nil {
		return false, err
	}
	expected, err := marshaler(&e)
	if err != nil {
		return false, err
	}
	if patcher == nil {
		patcher = defaultPatcher
	}
	patch, err := patcher(actual, expected)
	if err != nil {
		return false, err
	}
	return len(patch) == 2, nil
}
