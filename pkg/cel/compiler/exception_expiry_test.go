package compiler

import (
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompiledExceptionIsExpired(t *testing.T) {
	past := metav1.NewTime(time.Now().Add(-time.Hour))
	future := metav1.NewTime(time.Now().Add(time.Hour))
	for _, tt := range []struct {
		name    string
		expiry  *metav1.Time
		expired bool
	}{
		{"never", nil, false},
		{"future", &future, false},
		{"past", &past, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := Exception{Exception: &policiesv1beta1.PolicyException{Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: tt.expiry}}}
			if got := e.IsExpired(); got != tt.expired {
				t.Fatalf("IsExpired()=%v, want %v", got, tt.expired)
			}
		})
	}
	if (Exception{}).IsExpired() {
		t.Fatal("nil source exception should not appear expired")
	}
}

// TestCompiledExceptionExpiryClock verifies the deadline remains unchanged
// when evaluation time advances. No global clock mutation is required.
func TestCompiledExceptionExpiryClock(t *testing.T) {
	deadline := metav1.NewTime(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))
	now := deadline.Time.Add(-time.Second)
	exception := Exception{
		Exception: &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{ExpiresAt: &deadline},
		},
		Now: func() time.Time { return now },
	}
	if exception.IsExpired() {
		t.Fatal("exception expired before its unchanged deadline")
	}
	now = deadline.Time.Add(time.Second)
	if !exception.IsExpired() {
		t.Fatal("exception remained active after its unchanged deadline")
	}
}
