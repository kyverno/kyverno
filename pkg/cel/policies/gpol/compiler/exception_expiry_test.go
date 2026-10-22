package compiler

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			result, err := p.Evaluate(context.Background(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
			if err != nil || result == nil {
				t.Fatalf("evaluate result=%v error=%v", result, err)
			}
			if got := len(result.Exceptions) > 0; got != tt.exempt {
				t.Fatalf("exempt=%v, want %v", got, tt.exempt)
			}
		})
	}
}

// The same compiled generating policy is evaluated on both sides of a fixed
// deadline; only the injected evaluation time changes.
func TestCachedPolicyExceptionExpiryAcrossEvaluations(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	deadline := metav1.NewTime(now.Add(time.Minute))
	exception := &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &deadline}}
	// Compile an actual generation rather than asserting only that the
	// expired exception disappears from the evaluation result.
	generating := &policiesv1beta1.GeneratingPolicy{
		Spec: policiesv1beta1.GeneratingPolicySpec{
			Generation: []policiesv1beta1.Generation{{
				Expression: `generator.Apply("target-ns", [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "expiry-regression"})}])`,
			}},
		},
	}
	policy, errs := NewCompiler().Compile(generating, []*policiesv1beta1.PolicyException{exception})
	if errs != nil {
		t.Fatalf("compiling generation with exception: %v", errs)
	}
	if len(policy.exceptions) != 1 {
		t.Fatalf("expected one compiled exception, got %d", len(policy.exceptions))
	}
	policy.exceptions[0].Now = func() time.Time { return now }
	before, err := policy.Evaluate(context.Background(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
	if err != nil || before == nil || len(before.Exceptions) != 1 || len(before.GeneratedResources) != 0 {
		t.Fatalf("active exception unexpectedly allowed generation: result=%#v err=%v", before, err)
	}
	now = now.Add(2 * time.Minute)
	after, err := policy.Evaluate(context.Background(), attr, &request.Request, &ns, &libs.FakeContextProvider{})
	if err != nil || after == nil || len(after.Exceptions) != 0 || len(after.GeneratedResources) != 1 {
		t.Fatalf("expired cached exception suppressed resource generation: result=%#v err=%v", after, err)
	}
	if after.GeneratedResources[0].GetKind() != "ConfigMap" || after.GeneratedResources[0].GetName() != "expiry-regression" {
		t.Fatalf("unexpected generated resource after exception expiry: %#v", after.GeneratedResources[0])
	}
}
