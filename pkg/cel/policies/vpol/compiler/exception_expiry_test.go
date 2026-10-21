package compiler

import (
	"context"
	"testing"
	"time"

	"github.com/google/cel-go/common/types"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCachedPolicyExceptionExpiry(t *testing.T) {
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	future := metav1.NewTime(time.Now().Add(time.Hour))
	makeEx := func(name string, expiry *metav1.Time) celcompiler.Exception {
		return celcompiler.Exception{Exception: &policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: expiry}}}
	}
	for _, tt := range []struct {
		name       string
		exceptions []celcompiler.Exception
		exempt     bool
	}{
		{"future", []celcompiler.Exception{makeEx("future", &future)}, true},
		{"no expiration", []celcompiler.Exception{makeEx("permanent", nil)}, true},
		{"expired", []celcompiler.Exception{makeEx("expired", &past)}, false},
		{"mixed", []celcompiler.Exception{makeEx("expired", &past), makeEx("future", &future)}, true},
		{"all expired", []celcompiler.Exception{makeEx("a", &past), makeEx("b", &past)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{exceptions: tt.exceptions, validations: []celcompiler.Validation{{Program: &mockVpolProgram{retVal: types.Bool(false)}, Message: "deny"}}}
			result, err := p.evaluateWithData(context.Background(), evaluationData{})
			if err != nil || result == nil {
				t.Fatalf("evaluate returned %v, %v", result, err)
			}
			if got := len(result.Exceptions) > 0; got != tt.exempt {
				t.Fatalf("exempt=%v, want %v", got, tt.exempt)
			}
			if !tt.exempt && (result.Result || result.Message != "deny") {
				t.Fatalf("expired exception bypassed validation: %#v", result)
			}
			for _, ex := range result.Exceptions {
				if ex.IsExpired() {
					t.Fatalf("expired exception %s retained", ex.Name)
				}
			}
		})
	}
}

// TestCachedPolicyExceptionExpiryAcrossEvaluations reuses the same compiled
// policy while its referenced PolicyException crosses the expiry boundary.
func TestCachedPolicyExceptionExpiryAcrossEvaluations(t *testing.T) {
	expiry := metav1.NewTime(time.Now().Add(time.Hour))
	exception := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &expiry},
	}
	policy := &Policy{
		exceptions: []celcompiler.Exception{{Exception: exception}},
		validations: []celcompiler.Validation{{
			Program: &mockVpolProgram{retVal: types.Bool(false)},
			Message: "deny",
		}},
	}
	before, err := policy.evaluateWithData(context.Background(), evaluationData{})
	if err != nil || before == nil || len(before.Exceptions) != 1 {
		t.Fatalf("active exception not honored: result=%#v err=%v", before, err)
	}
	// No compilation or informer update occurs before the second evaluation.
	expiry.Time = time.Now().Add(-time.Hour)
	after, err := policy.evaluateWithData(context.Background(), evaluationData{})
	if err != nil || after == nil {
		t.Fatalf("evaluation after expiration failed: result=%#v err=%v", after, err)
	}
	if len(after.Exceptions) != 0 || after.Result || after.Message != "deny" {
		t.Fatalf("cached expired exception bypassed enforcement: %#v", after)
	}
}
