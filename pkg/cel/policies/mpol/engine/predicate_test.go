package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	v1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		policies []policiesv1beta1.MutatingPolicyLike
		expected []bool
	}{
		{
			name:     "no names provided - always match",
			names:    []string{},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "any"}}},
			expected: []bool{true, true},
		},
		{
			name:     "single name match",
			names:    []string{"p1"},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p2"}}},
			expected: []bool{true, false},
		},
		{
			name:     "multiple name match",
			names:    []string{"p1", "p3"},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p2"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p3"}}},
			expected: []bool{true, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := MatchNames(tt.names...)
			for i, policy := range tt.policies {
				assert.Equal(t, tt.expected[i], predicate(policy), "policy name: %s", policy.GetName())
			}
		})
	}
}

func makeMutatingPolicy(name, namespace string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
}

func TestClusteredPolicy(t *testing.T) {
	pred := ClusteredPolicy()
	assert.True(t, pred(makeMutatingPolicy("cluster-pol", "")))
	assert.False(t, pred(makeMutatingPolicy("ns-pol", "default")))
}

func TestNamespacedPolicy(t *testing.T) {
	pred := NamespacedPolicy("default")
	assert.True(t, pred(makeMutatingPolicy("ns-pol", "default")))
	assert.False(t, pred(makeMutatingPolicy("other", "kube-system")))
	assert.False(t, pred(makeMutatingPolicy("cluster-pol", "")))
}

func TestNoTargetMatchConstraintPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy *policiesv1beta1.MutatingPolicy
		want   bool
	}{{
		name:   "nil target match constraints",
		policy: makeMutatingPolicy("p", ""),
		want:   true,
	}, {
		name: "empty target match constraints",
		policy: &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "p"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &v1alpha1.TargetMatchConstraints{},
			},
		},
		want: true,
	}, {
		name: "with resource rules",
		policy: &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "p"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &v1alpha1.TargetMatchConstraints{
					MatchResources: admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Rule: admissionregistrationv1.Rule{
									Resources: []string{"pods"},
								},
							},
						}},
					},
				},
			},
		},
		want: false,
	}, {
		name: "with expression",
		policy: &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "p"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				TargetMatchConstraints: &v1alpha1.TargetMatchConstraints{
					Expression: `object.metadata.name == "test"`,
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NoTargetMatchConstraintPolicy()
			assert.Equal(t, tt.want, pred(tt.policy))
		})
	}
}

func TestAnd(t *testing.T) {
	alwaysTrue := func(policiesv1beta1.MutatingPolicyLike) bool { return true }
	alwaysFalse := func(policiesv1beta1.MutatingPolicyLike) bool { return false }

	p := makeMutatingPolicy("my-policy", "")
	assert.True(t, And()(p))
	assert.True(t, And(alwaysTrue)(p))
	assert.False(t, And(alwaysFalse)(p))
	assert.True(t, And(alwaysTrue, alwaysTrue)(p))
	assert.False(t, And(alwaysFalse, alwaysTrue)(p))
	assert.False(t, And(alwaysTrue, alwaysFalse)(p))
	assert.True(t, And(nil, alwaysTrue)(p))
}

func TestOr(t *testing.T) {
	alwaysTrue := func(policiesv1beta1.MutatingPolicyLike) bool { return true }
	alwaysFalse := func(policiesv1beta1.MutatingPolicyLike) bool { return false }

	p := makeMutatingPolicy("my-policy", "")
	assert.False(t, Or()(p))
	assert.True(t, Or(alwaysTrue)(p))
	assert.False(t, Or(alwaysFalse)(p))
	assert.True(t, Or(alwaysTrue, alwaysFalse)(p))
	assert.True(t, Or(alwaysFalse, alwaysTrue)(p))
	assert.False(t, Or(alwaysFalse, alwaysFalse)(p))
	assert.False(t, Or(nil, alwaysFalse)(p))
	assert.True(t, Or(nil, alwaysTrue)(p))
}
