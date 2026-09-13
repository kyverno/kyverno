package context

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

var benchPodResourceRaw = []byte(`{
	"apiVersion": "v1",
	"kind": "Pod",
	"metadata": {
		"name": "myapp-pod",
		"namespace": "default",
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

// BenchmarkContextCheckpointRestore measures the JSON deep-copy cost of
// Checkpoint (and the matching Restore) on a context loaded with a
// representative pod payload. Restore is paired with Checkpoint in the loop
// so the checkpoint stack does not grow unboundedly across iterations, which
// would otherwise skew B/op upward as b.N increases.
func BenchmarkContextCheckpointRestore(b *testing.B) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)
	if err := AddResource(ctx, benchPodResourceRaw); err != nil {
		b.Fatalf("failed to add resource: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		ctx.Restore()
	}
}
