package compiler

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCachedPolicyExceptionExpiry(t *testing.T) {
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	future := metav1.NewTime(time.Now().Add(time.Hour))
	for _, tt := range []struct {
		name   string
		expiry *metav1.Time
		exempt bool
	}{
		{"expired", &past, false}, {"future", &future, true}, {"permanent", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ex := &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: tt.expiry}}
			p := &Policy{exceptions: []celcompiler.Exception{{Exception: ex}}}
			result, err := p.Evaluate(context.Background(), unstructured.Unstructured{}, &unstructured.Unstructured{}, &libs.FakeContextProvider{})
			if err != nil || result == nil {
				t.Fatalf("evaluate result=%v error=%v", result, err)
			}
			if got := len(result.Exceptions) > 0; got != tt.exempt {
				t.Fatalf("exempt=%v, want %v", got, tt.exempt)
			}
			if !tt.exempt && !result.Result {
				t.Fatal("expired exception prevented deletion")
			}
		})
	}
}

// A cached deleting policy must stop honoring the same exception when time,
// rather than the exception object, advances beyond the unchanged deadline.
func TestCachedPolicyExceptionExpiryAcrossEvaluations(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	deadline := metav1.NewTime(now.Add(time.Minute))
	exception := &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &deadline}}
	policy := &Policy{exceptions: []celcompiler.Exception{{
		Exception: exception,
		Now: func() time.Time { return now },
	}}}
	before, err := policy.Evaluate(context.Background(), unstructured.Unstructured{}, &unstructured.Unstructured{}, &libs.FakeContextProvider{})
	if err != nil || before == nil || len(before.Exceptions) != 1 {
		t.Fatalf("active exception not honored: result=%#v err=%v", before, err)
	}
	now = now.Add(2 * time.Minute)
	after, err := policy.Evaluate(context.Background(), unstructured.Unstructured{}, &unstructured.Unstructured{}, &libs.FakeContextProvider{})
	if err != nil || after == nil {
		t.Fatalf("evaluation after expiry failed: result=%#v err=%v", after, err)
	}
	if len(after.Exceptions) != 0 || !after.Result {
		t.Fatalf("expired cached exception prevented deletion: %#v", after)
	}
}
