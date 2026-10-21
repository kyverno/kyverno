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
