package engine

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type fakeGpolLister struct {
	policy *policiesv1beta1.GeneratingPolicy
	err    error
}

func (f *fakeGpolLister) Get(name string) (*policiesv1beta1.GeneratingPolicy, error) {
	return f.policy, f.err
}

func (f *fakeGpolLister) List(selector labels.Selector) ([]*policiesv1beta1.GeneratingPolicy, error) {
	return nil, nil
}

type fakeNgpolLister struct {
	// policies, if set, is served by NamespacedGeneratingPolicies(ns).Get(name); unset falls back
	// to a nil per-namespace lister, matching the old always-nil behaviour other subtests rely on.
	policies map[string]*policiesv1beta1.NamespacedGeneratingPolicy
}

func (f *fakeNgpolLister) List(selector labels.Selector) ([]*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	return nil, nil
}

func (f *fakeNgpolLister) NamespacedGeneratingPolicies(namespace string) policiesv1beta1listers.NamespacedGeneratingPolicyNamespaceLister {
	if f.policies == nil {
		return nil
	}
	return &fakeNgpolNamespaceLister{namespace: namespace, policies: f.policies}
}

type fakeNgpolNamespaceLister struct {
	namespace string
	policies  map[string]*policiesv1beta1.NamespacedGeneratingPolicy
}

func (f *fakeNgpolNamespaceLister) List(labels.Selector) ([]*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	return nil, nil
}

func (f *fakeNgpolNamespaceLister) Get(name string) (*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	if p, ok := f.policies[f.namespace+"/"+name]; ok {
		return p, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
}

type fakePolexLister struct {
	exceptions []*policiesv1beta1.PolicyException
	err        error
}

func (f *fakePolexLister) List(_ labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return f.exceptions, f.err
}

func (f *fakePolexLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	if f.err != nil {
		return nil, f.err
	}
	return engine.ExceptionScopeAll.Select(f.exceptions, kind, name, namespace), nil
}

func TestGet(t *testing.T) {
	t.Run("", func(t *testing.T) {
		comp := compiler.NewCompiler()
		gpol := &policiesv1beta1.GeneratingPolicy{
			ObjectMeta: v1.ObjectMeta{
				Name: "test-policy",
			},
		}
		gpol.TypeMeta.Kind = "GeneratingPolicy"

		exception := &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{
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
			&fakeNgpolLister{},
			&fakePolexLister{exceptions: []*policiesv1beta1.PolicyException{exception}},
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
			&fakeNgpolLister{},
			&fakePolexLister{exceptions: []*policiesv1beta1.PolicyException{nil}},
			true,
		)

		_, err := fp.Get(context.Background(), "test-policy")
		assert.Error(t, err)
	})

	t.Run("", func(t *testing.T) {
		comp := compiler.NewCompiler()
		// GetKind/GetName/GetNamespace are evaluated as ListForPolicy's arguments, so the
		// policy must be non-nil even though only the exception lookup is under test here.
		gpol := &policiesv1beta1.GeneratingPolicy{ObjectMeta: v1.ObjectMeta{Name: "test-policy"}}
		gpol.TypeMeta.Kind = "GeneratingPolicy"

		fp := NewFetchProvider(
			comp,
			&fakeGpolLister{policy: gpol},
			&fakeNgpolLister{},
			&fakePolexLister{err: errors.New("error while test")},
			true,
		)

		_, err := fp.Get(context.Background(), "test-policy")
		assert.Error(t, err)
	})
}

// TestGet_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument passed to
// ListForPolicy: under scope "*" a namespaced policy must only accept exceptions filed in its own
// namespace, never a foreign one. It fails if Get ever stops passing policy.GetNamespace() through
// to the exception lookup.
func TestGet_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{ObjectMeta: v1.ObjectMeta{Name: "guard", Namespace: "tenant-a"}}
	ngpol.TypeMeta.Kind = "NamespacedGeneratingPolicy"
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: v1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: v1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}

	fp := NewFetchProvider(
		compiler.NewCompiler(),
		&fakeGpolLister{},
		&fakeNgpolLister{policies: map[string]*policiesv1beta1.NamespacedGeneratingPolicy{"tenant-a/guard": ngpol}},
		&fakePolexLister{exceptions: []*policiesv1beta1.PolicyException{ownException, foreignException}},
		true,
	)

	policy, err := fp.Get(context.Background(), "tenant-a/guard")
	require.NoError(t, err)
	require.Len(t, policy.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", policy.Exceptions[0].GetNamespace())
	assert.Equal(t, "own", policy.Exceptions[0].GetName())
}

// TestGet_ScopeGatesExceptionSelection pins the scope used by the polexLister Get relies on for
// namespace filtering: an empty scope ("--exceptionNamespace" unset) must select nothing, and a
// concrete scope must select only the exception filed in that (central) namespace, even though
// the policy's own namespace would otherwise qualify. Unlike the other fakes in this file,
// fakePolexLister hardcodes ExceptionScopeAll, so this exercises the real production lister
// (engine.NewPolicyExceptionLister) through NewFetchProvider's Get instead.
func TestGet_ScopeGatesExceptionSelection(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{ObjectMeta: v1.ObjectMeta{Name: "guard", Namespace: "tenant-a"}}
	ngpol.TypeMeta.Kind = "NamespacedGeneratingPolicy"
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: v1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}
	centralException := &policiesv1beta1.PolicyException{
		ObjectMeta: v1.ObjectMeta{Name: "central", Namespace: "central-ns"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedGeneratingPolicy"}},
		},
	}
	polexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	require.NoError(t, polexIndexer.Add(ownException))
	require.NoError(t, polexIndexer.Add(centralException))

	tests := []struct {
		name          string
		scope         string
		expectedNames []string
	}{
		{name: "scope none selects nothing", scope: ""},
		{name: "concrete scope selects only the central namespace's exception", scope: "central-ns", expectedNames: []string{"central"}},
	}

	comp := compiler.NewCompiler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			polexLister := engine.NewPolicyExceptionLister(policiesv1beta1listers.NewPolicyExceptionLister(polexIndexer), tt.scope)
			fp := NewFetchProvider(
				comp,
				&fakeGpolLister{},
				&fakeNgpolLister{policies: map[string]*policiesv1beta1.NamespacedGeneratingPolicy{"tenant-a/guard": ngpol}},
				polexLister,
				true,
			)

			policy, err := fp.Get(context.Background(), "tenant-a/guard")
			require.NoError(t, err)

			if len(tt.expectedNames) == 0 {
				assert.Empty(t, policy.Exceptions)
				return
			}
			require.Len(t, policy.Exceptions, len(tt.expectedNames))
			for i, name := range tt.expectedNames {
				assert.Equal(t, name, policy.Exceptions[i].GetName())
			}
		})
	}
}
