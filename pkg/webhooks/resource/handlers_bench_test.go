package resource

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	log "github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
)

// BenchmarkHandlerValidate measures the full webhook Validate path -- request
// unmarshal, cache lookup, policy-context build, engine validate, metrics
// recording -- using NewFakeHandlers (no cluster).
//
// The seeded policy uses Enforce mode against a resource (goodPod) that
// satisfies its pattern, so every iteration takes the synchronous
// allow-with-no-audit-followup path deterministically.
//
// Known noise source: Validate submits async audit work to a pond pool and
// the fake informers run background goroutines, which can make allocation
// counts on this benchmark noisier than the engine/cache benchmarks. Verify
// stability with `go test -bench=BenchmarkHandlerValidate -benchmem -count=5`
// before trusting a ceiling change; this benchmark is gated with wider
// (+20%) allocs/op headroom in scripts/bench/thresholds.txt to absorb that
// noise.
func BenchmarkHandlerValidate(b *testing.B) {
	logger := log.WithName("BenchmarkHandlerValidate")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	policyCache := policycache.NewCache()
	resourceHandlers := NewFakeHandlers(ctx, policyCache)

	var validPolicy kyverno.ClusterPolicy
	if err := json.Unmarshal([]byte(policyCheckLabel), &validPolicy); err != nil {
		b.Fatalf("failed to unmarshal policy: %v", err)
	}
	validPolicy.Spec.ValidationFailureAction = "Enforce"

	key := makeKey(&validPolicy)
	if err := policyCache.Set(key, &validPolicy, policycache.TestResourceFinder{}); err != nil {
		b.Fatalf("failed to seed policy cache: %v", err)
	}

	request := handlers.AdmissionRequest{
		AdmissionRequest: v1.AdmissionRequest{
			Operation: v1.Create,
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Object: apiruntime.RawExtension{
				Raw: []byte(goodPod),
			},
			RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = resourceHandlers.Validate(ctx, logger, request, "", time.Now())
	}
}
