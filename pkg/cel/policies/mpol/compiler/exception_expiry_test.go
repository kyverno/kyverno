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
