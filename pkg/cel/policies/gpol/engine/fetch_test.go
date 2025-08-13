package engine

import (
	"context"
	"errors"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type fakeGpolLister struct {
	policy *policiesv1alpha1.GeneratingPolicy
	err    error
}

func (f *fakeGpolLister) Get(name string) (*policiesv1alpha1.GeneratingPolicy, error) {
	return f.policy, f.err
}

func (f *fakeGpolLister) List(selector labels.Selector) ([]*policiesv1alpha1.GeneratingPolicy, error) {
	return nil, nil
}

type fakePolexLister struct {
	exceptions []*policiesv1alpha1.PolicyException
	err        error
}

func (f *fakePolexLister) List(_ labels.Selector) ([]*policiesv1alpha1.PolicyException, error) {
	return f.exceptions, f.err
}
func (f *fakePolexLister) PolicyExceptions(namespace string) v1alpha1.PolicyExceptionNamespaceLister {
	return nil
}

func TestGet(t *testing.T) {
	t.Run("", func(t *testing.T) {
		comp := compiler.NewCompiler()
		gpol := &policiesv1alpha1.GeneratingPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name: "test-policy",
			},
		}
		gpol.TypeMeta.Kind = "GeneratingPolicy"

		exception := &policiesv1alpha1.PolicyException{
			Spec: policiesv1alpha1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1alpha1.PolicyRef{
					{
						Name: "test-policy",
						Kind: "GeneratingPolicy",
					},
				},
			},
		}

		fp := NewFetchProvider(
			comp,
			&fakeGpolLister{policy: gpol},
			&fakePolexLister{exceptions: []*policiesv1alpha1.PolicyException{exception}},
			true,
		)

		policy, err := fp.Get(context.Background(), "test-policy")
		assert.NoError(t, err)
		assert.Equal(t, "test-policy", policy.Policy.GetName())
		assert.Len(t, policy.Exceptions, 1)
		assert.NotNil(t, policy.CompiledPolicy)
	})

	t.Run("", func(t *testing.T) {
		comp := compiler.NewCompiler()

		fp := NewFetchProvider(
			comp,
			&fakeGpolLister{err: errors.New("forced error")},
			&fakePolexLister{exceptions: []*policiesv1alpha1.PolicyException{nil}},
			true,
		)

		_, err := fp.Get(context.Background(), "test-policy")
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		comp := compiler.NewCompiler()

		fp := NewFetchProvider(
			comp,
			&fakeGpolLister{policy: nil},
			&fakePolexLister{err: errors.New("error while test")},
			true,
		)

		_, err := fp.Get(context.Background(), "test-policy")
		assert.Error(t, err)
	})
}
