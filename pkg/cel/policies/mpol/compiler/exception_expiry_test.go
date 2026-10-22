package compiler

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
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
			p := &Policy{exceptions: []celcompiler.Exception{{Exception: ex}}, patchers: []Patcher{&fakePatcher{retVal: &unstructured.Unstructured{}}}}
			result := p.Evaluate(context.Background(), &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
			if result == nil || result.Error != nil {
				t.Fatalf("evaluation failed: %#v", result)
			}
			if got := len(result.Exceptions) > 0; got != tt.exempt {
				t.Fatalf("exempt=%v, want %v", got, tt.exempt)
			}
			if !tt.exempt && result.PatchedResource == nil {
				t.Fatal("expired exemption suppressed patch")
			}
		})
	}
}

// Reuse an unchanged compiled mutating policy across a fixed expiration
// boundary; the mutation must run once the exception is no longer active.
func TestCachedPolicyExceptionExpiryAcrossEvaluations(t *testing.T) {
	now := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	deadline := metav1.NewTime(now.Add(time.Minute))
	exception := &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &deadline}}
	policy := &Policy{
		exceptions: []celcompiler.Exception{{
			Exception: exception,
			Now: func() time.Time { return now },
		}},
		patchers: []Patcher{&fakePatcher{retVal: &unstructured.Unstructured{}}},
	}
	before := policy.Evaluate(context.Background(), &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
	if before == nil || before.Error != nil || len(before.Exceptions) != 1 {
		t.Fatalf("active exception not honored: %#v", before)
	}
	now = now.Add(2 * time.Minute)
	after := policy.Evaluate(context.Background(), &mockAttributes{}, &corev1.Namespace{}, admissionv1.AdmissionRequest{}, &fakeTCM{}, nil, &libs.FakeContextProvider{})
	if after == nil || after.Error != nil || len(after.Exceptions) != 0 || after.PatchedResource == nil {
		t.Fatalf("expired cached exception suppressed mutation: %#v", after)
	}
}
