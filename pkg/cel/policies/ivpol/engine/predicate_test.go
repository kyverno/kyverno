package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeImagePolicy(name string) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name       string
		names      []string
		policyName string
		want       bool
	}{{
		name:       "nil names matches everything",
		names:      nil,
		policyName: "any-policy",
		want:       true,
	}, {
		name:       "empty names matches everything",
		names:      []string{},
		policyName: "any-policy",
		want:       true,
	}, {
		name:       "single name matches",
		names:      []string{"my-policy"},
		policyName: "my-policy",
		want:       true,
	}, {
		name:       "single name does not match",
		names:      []string{"my-policy"},
		policyName: "other-policy",
		want:       false,
	}, {
		name:       "multiple names matches first",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-a",
		want:       true,
	}, {
		name:       "multiple names matches second",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-b",
		want:       true,
	}, {
		name:       "multiple names no match",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-c",
		want:       false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := MatchNames(tt.names...)
			got := pred(makeImagePolicy(tt.policyName))
			assert.Equal(t, tt.want, got)
		})
	}
}
