package internal

import (
	"reflect"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAcceptedAPIVersionsAcceptsV1beta1AndV1(t *testing.T) {
	assert.True(t, IsAcceptedAPIVersion("policies.kyverno.io/v1beta1"))
	assert.True(t, IsAcceptedAPIVersion("policies.kyverno.io/v1"))
}

func TestAcceptedAPIVersionsRejectsV1alpha1(t *testing.T) {
	assert.False(t, IsAcceptedAPIVersion("policies.kyverno.io/v1alpha1"))
}

func TestAcceptedAPIVersionsRejectsUnrelatedGroup(t *testing.T) {
	assert.False(t, IsAcceptedAPIVersion("kyverno.io/v1"))
}

func TestAcceptedAPIVersionsTextNamesBoth(t *testing.T) {
	text := AcceptedAPIVersionsText()
	assert.Contains(t, text, "policies.kyverno.io/v1beta1")
	assert.Contains(t, text, "policies.kyverno.io/v1")
}

// TestAcceptedAPIVersionsAgreeWithExceptionPredicates binds AcceptedAPIVersions (D4) to
// cmd/cli/kubectl-kyverno/exception's canonical GVK predicates: every accepted version must
// satisfy exception.IsCELException for the PolicyException kind, and v1alpha1 must not.
func TestAcceptedAPIVersionsAgreeWithExceptionPredicates(t *testing.T) {
	for _, v := range AcceptedAPIVersions {
		gv, err := schema.ParseGroupVersion(v)
		assert.NoError(t, err)
		gvk := gv.WithKind("PolicyException")
		assert.True(t, exception.IsCELException(gvk), "accepted version %s must be a CEL exception GVK", v)
	}
	rejected := schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("PolicyException")
	assert.False(t, IsAcceptedAPIVersion(rejected.GroupVersion().String()))
}

// TestV1AndV1beta1JSONSchemasAgree pins the safety invariant the decision log's "type aliases"
// correction rests on: v1 and v1beta1 are distinct Go structs, but their JSON tag sets must be
// identical for a writer that accepts both without rewriting apiVersion to stay safe. Reflection
// catches a schema drift a prose comment would not.
func TestV1AndV1beta1JSONSchemasAgree(t *testing.T) {
	cases := []struct {
		name        string
		v1, v1beta1 interface{}
	}{
		{"ValidatingPolicy", policiesv1.ValidatingPolicy{}, policiesv1beta1.ValidatingPolicy{}},
		{"MutatingPolicy", policiesv1.MutatingPolicy{}, policiesv1beta1.MutatingPolicy{}},
		{"GeneratingPolicy", policiesv1.GeneratingPolicy{}, policiesv1beta1.GeneratingPolicy{}},
		{"DeletingPolicy", policiesv1.DeletingPolicy{}, policiesv1beta1.DeletingPolicy{}},
		{"ImageValidatingPolicy", policiesv1.ImageValidatingPolicy{}, policiesv1beta1.ImageValidatingPolicy{}},
		{"PolicyException", policiesv1.PolicyException{}, policiesv1beta1.PolicyException{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v1Tags := jsonTags(reflect.TypeOf(c.v1))
			v1beta1Tags := jsonTags(reflect.TypeOf(c.v1beta1))
			assert.Equal(t, v1beta1Tags, v1Tags, "%s: v1 and v1beta1 JSON schemas must agree", c.name)
		})
	}
}

// jsonTags recursively collects every "json" struct tag reachable from t, so an added, removed,
// or renamed field in either version's struct shows up as a diff, not just a spot check.
func jsonTags(t reflect.Type) []string {
	var tags []string
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tags = append(tags, f.Tag.Get("json"))
			ft := f.Type
			for ft.Kind() == reflect.Ptr || ft.Kind() == reflect.Slice {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				walk(ft)
			}
		}
	}
	walk(t)
	return tags
}
