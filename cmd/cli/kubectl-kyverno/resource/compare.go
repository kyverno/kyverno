package resource

import (
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
