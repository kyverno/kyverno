package engine

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type fakeNgpolLister struct{}

func (f *fakeNgpolLister) List(selector labels.Selector) ([]*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	return nil, nil
}

func (f *fakeNgpolLister) NamespacedGeneratingPolicies(namespace string) policiesv1beta1listers.NamespacedGeneratingPolicyNamespaceLister {
	return nil
}

// nameAwareGpolLister answers by name and reports a miss with a NotFound error,
// matching the generated lister contract.
type nameAwareGpolLister struct {
	policies map[string]*policiesv1beta1.GeneratingPolicy
}

func (f *nameAwareGpolLister) Get(name string) (*policiesv1beta1.GeneratingPolicy, error) {
	if policy, ok := f.policies[name]; ok {
		return policy, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "policies.kyverno.io", Resource: "generatingpolicies"}, name)
}

func (f *nameAwareGpolLister) List(selector labels.Selector) ([]*policiesv1beta1.GeneratingPolicy, error) {
	policies := make([]*policiesv1beta1.GeneratingPolicy, 0, len(f.policies))
	for _, policy := range f.policies {
		policies = append(policies, policy)
	}
	return policies, nil
}

// nameAwareNgpolLister answers by namespace and name, matching the generated
// lister contract.
type nameAwareNgpolLister struct {
	policies []*policiesv1beta1.NamespacedGeneratingPolicy
}

func (f *nameAwareNgpolLister) List(selector labels.Selector) ([]*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	return f.policies, nil
}

func (f *nameAwareNgpolLister) NamespacedGeneratingPolicies(namespace string) policiesv1beta1listers.NamespacedGeneratingPolicyNamespaceLister {
	return &nameAwareNgpolNamespaceLister{namespace: namespace, policies: f.policies}
}

type nameAwareNgpolNamespaceLister struct {
	namespace string
	policies  []*policiesv1beta1.NamespacedGeneratingPolicy
}

func (l *nameAwareNgpolNamespaceLister) Get(name string) (*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	for _, policy := range l.policies {
		if policy.GetNamespace() == l.namespace && policy.GetName() == name {
			return policy, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "policies.kyverno.io", Resource: "namespacedgeneratingpolicies"}, name)
}

func (l *nameAwareNgpolNamespaceLister) List(selector labels.Selector) ([]*policiesv1beta1.NamespacedGeneratingPolicy, error) {
	policies := make([]*policiesv1beta1.NamespacedGeneratingPolicy, 0, len(l.policies))
	for _, policy := range l.policies {
		if policy.GetNamespace() == l.namespace {
			policies = append(policies, policy)
		}
	}
	return policies, nil
}

type fakePolexLister struct {
	exceptions []*policiesv1beta1.PolicyException
	err        error
}

func clusterGeneratingPolicy(name string) *policiesv1beta1.GeneratingPolicy {
	policy := &policiesv1beta1.GeneratingPolicy{ObjectMeta: v1.ObjectMeta{Name: name}}
	policy.TypeMeta.Kind = "GeneratingPolicy"
	return policy
}

func namespacedGeneratingPolicy(namespace, name string) *policiesv1beta1.NamespacedGeneratingPolicy {
	policy := &policiesv1beta1.NamespacedGeneratingPolicy{ObjectMeta: v1.ObjectMeta{Namespace: namespace, Name: name}}
	policy.TypeMeta.Kind = "NamespacedGeneratingPolicy"
	return policy
}

// A downstream generated before the policy namespace label existed carries no
// owner, so a same-named policy in the other scope makes migration unsafe for
// both of them.
func TestGet_LegacyOwnerConflict(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		gpols     map[string]*policiesv1beta1.GeneratingPolicy
		ngpols    []*policiesv1beta1.NamespacedGeneratingPolicy
		conflicts map[string]bool
	}{
		{
			name:  "cluster policy only",
			key:   "pol",
			gpols: map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
		},
		{
			name:   "namespaced policy only",
			key:    "tenant-ns/pol",
			ngpols: []*policiesv1beta1.NamespacedGeneratingPolicy{namespacedGeneratingPolicy("tenant-ns", "pol")},
		},
		{
			name: "same name in another namespace is not a conflict",
			key:  "tenant-ns/pol",
			ngpols: []*policiesv1beta1.NamespacedGeneratingPolicy{
				namespacedGeneratingPolicy("tenant-ns", "pol"),
				namespacedGeneratingPolicy("other-ns", "pol"),
			},
		},
		{
			name:   "cluster policy with a differently named namespaced policy",
			key:    "pol",
			gpols:  map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
			ngpols: []*policiesv1beta1.NamespacedGeneratingPolicy{namespacedGeneratingPolicy("tenant-ns", "other")},
		},
		{
			name:      "namespaced policy shadowed by a cluster policy",
			key:       "tenant-ns/pol",
			gpols:     map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
			ngpols:    []*policiesv1beta1.NamespacedGeneratingPolicy{namespacedGeneratingPolicy("tenant-ns", "pol")},
			conflicts: map[string]bool{"tenant-ns": true},
		},
		{
			name:      "cluster policy shadowed only in the namespace hosting the namespaced policy",
			key:       "pol",
			gpols:     map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
			ngpols:    []*policiesv1beta1.NamespacedGeneratingPolicy{namespacedGeneratingPolicy("tenant-ns", "pol")},
			conflicts: map[string]bool{"tenant-ns": true},
		},
		{
			name:  "cluster policy shadowed in another namespace only",
			key:   "pol",
			gpols: map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
			ngpols: []*policiesv1beta1.NamespacedGeneratingPolicy{
				namespacedGeneratingPolicy("other-ns", "pol"),
			},
			conflicts: map[string]bool{"other-ns": true},
		},
		{
			name:  "cluster policy shadowed in several namespaces",
			key:   "pol",
			gpols: map[string]*policiesv1beta1.GeneratingPolicy{"pol": clusterGeneratingPolicy("pol")},
			ngpols: []*policiesv1beta1.NamespacedGeneratingPolicy{
				namespacedGeneratingPolicy("tenant-ns", "pol"),
				namespacedGeneratingPolicy("other-ns", "pol"),
			},
			conflicts: map[string]bool{"tenant-ns": true, "other-ns": true},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fp := NewFetchProvider(
				compiler.NewCompiler(),
				&nameAwareGpolLister{policies: test.gpols},
				&nameAwareNgpolLister{policies: test.ngpols},
				nil,
				false,
			)

			policy, err := fp.Get(context.Background(), test.key)
			require.NoError(t, err)
			assert.Equal(t, test.conflicts, policy.LegacyOwnerConflicts)
		})
	}
}

func (f *fakePolexLister) List(_ labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return f.exceptions, f.err
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

		fp := NewFetchProvider(
			comp,
			&fakeGpolLister{policy: nil},
			&fakeNgpolLister{},
			&fakePolexLister{err: errors.New("error while test")},
			true,
		)

		_, err := fp.Get(context.Background(), "test-policy")
		assert.Error(t, err)
	})
}
