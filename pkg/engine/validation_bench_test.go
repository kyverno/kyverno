package engine

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/metrics"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/registryclient"
)

// benchMetricsOnce forces the process-global metrics manager on for the
// engine benchmark, guarded so it only runs once per test binary and never
// from TestMain. This makes NewEngine's e.metrics non-nil, matching the
// production admission path (pkg/metrics/init.go wires SetManager during
// server bootstrap; plain `go test ./pkg/engine/...` never does). Because
// this only runs inside a Benchmark's setup, `make test-unit` (which uses
// `-run` without `-bench`) never executes it and unit-test behavior is
// unchanged.
var benchMetricsOnce sync.Once

func setupBenchMetrics() {
	benchMetricsOnce.Do(func() {
		metrics.SetManager(metrics.NewFakeMetricsConfig())
	})
}

// benchValidatePolicy is a single-rule enforce validate ClusterPolicy used to
// exercise the admission-hot-path Validate call.
var benchValidatePolicyRaw = []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
		"name": "validate-image"
	},
	"spec": {
		"validationFailureAction": "Enforce",
		"rules": [
			{
				"name": "validate-tag",
				"match": {
					"resources": {
						"kinds": ["Pod"]
					}
				},
				"validate": {
					"message": "An image tag is required",
					"pattern": {
						"spec": {
							"containers": [
								{
									"image": "*:*"
								}
							]
						}
					}
				}
			}
		]
	}
}`)

var benchPodResourceRaw = []byte(`{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "myapp-pod",
		"labels": {
			"app": "myapp"
		}
	},
	"spec": {
		"containers": [
			{
				"name": "nginx",
				"image": "nginx:latest",
				"imagePullPolicy": "Always"
			}
		]
	}
}`)

// BenchmarkEngineValidate measures a single-policy Validate pass through the
// engine on a representative pod resource, with the production metrics path
// forced on (see setupBenchMetrics). The engine is constructed once in setup
// so the benchmark measures the per-admission cost, not engine construction.
func BenchmarkEngineValidate(b *testing.B) {
	setupBenchMetrics()

	var policy kyvernov1.ClusterPolicy
	if err := json.Unmarshal(benchValidatePolicyRaw, &policy); err != nil {
		b.Fatalf("failed to unmarshal policy: %v", err)
	}

	resourceUnstructured, err := kubeutils.BytesToUnstructured(benchPodResourceRaw)
	if err != nil {
		b.Fatalf("failed to unmarshal resource: %v", err)
	}

	e := NewEngine(
		cfg,
		jp,
		nil,
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(registryclient.New()), nil),
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
		nil,
	)

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pContext := newPolicyContext(b, *resourceUnstructured, kyvernov1.Create, nil).WithPolicy(&policy)
		_ = e.Validate(ctx, pContext)
	}
}
