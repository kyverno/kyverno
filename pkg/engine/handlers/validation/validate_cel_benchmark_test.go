package validation

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func BenchmarkValidateCELHandlerProcess(b *testing.B) {
	handler, err := NewValidateCELHandler(newFakeClient(), true)
	if err != nil {
		b.Fatalf("NewValidateCELHandler() error = %v", err)
	}

	pc := buildCELContext(b, kyvernov1.Create, celPolicyPass, celPodResource, "")
	rule := pc.Policy().GetSpec().Rules[0]
	resource := pc.NewResource()
	logger := logr.Discard()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, responses := handler.Process(context.Background(), logger, pc, resource, rule, noopContextLoader, nil)
		if len(responses) != 1 {
			b.Fatalf("Process() returned %d responses, want 1", len(responses))
		}
	}
}
