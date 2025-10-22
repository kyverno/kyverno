package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
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
			&libs.FakeContextProvider{},
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

		data, err := prepareData(attr, &request.Request, &ns, &libs.FakeContextProvider{})
		assert.NotNil(t, data)
		assert.Nil(t, err)
	})
}
