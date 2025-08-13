package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

var (
	obj    = unstructured.Unstructured{}
	oldObj = unstructured.Unstructured{}
	ns     = unstructured.Unstructured{}
	res    = unstructured.Unstructured{}
)

// mock Context
type fakeContext struct{}

func (f *fakeContext) GenerateResources(string, []map[string]any) error        { return nil }
func (f *fakeContext) GetGlobalReference(name, projection string) (any, error) { return name, nil }
func (f *fakeContext) GetImageData(image string) (map[string]any, error) {
	return map[string]any{"test": image}, nil
}
func (f *fakeContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{}, nil
}
func (f *fakeContext) GetGeneratedResources() []*unstructured.Unstructured { return nil }
func (f *fakeContext) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}
func (f *fakeContext) ClearGeneratedResources() {}
func (f *fakeContext) SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	panic("not implemented")
}

func TestPrepareData(t *testing.T) {
	t.Run("valid-params", func(t *testing.T) {
		gvk := schema.GroupVersionKind{
			Group:   "",
			Version: "",
			Kind:    "",
		}
		res.SetGroupVersionKind(gvk)
		res.SetName("valid-name")
		res.SetNamespace("isolated-test")

		request := engine.Request(
			&fakeContext{},
			res.GroupVersionKind(),
			schema.GroupVersionResource{},
			"",
			res.GetName(),
			res.GetNamespace(),
			admissionv1.Create,
			authenticationv1.UserInfo{},
			&res,
			nil,
			false,
			nil,
		)

		attr := admission.NewAttributesRecord(
			&obj,
			&oldObj,
			schema.GroupVersionKind{},
			res.GetNamespace(),
			res.GetName(),
			res.GroupVersionKind().GroupVersion().WithResource(""),
			"",
			admission.Connect,
			&res,
			false,
			&user.DefaultInfo{},
		)

		data, err := prepareData(attr, &request.Request, &ns, &fakeContext{})
		assert.NotNil(t, data)
		assert.Nil(t, err)
	})
}
