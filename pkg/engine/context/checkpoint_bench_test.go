package context

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

func benchmarkPodContext(b *testing.B) Interface {
	b.Helper()
	cfg := config.NewDefaultConfiguration(false)
	ctx := NewContext(jmespath.New(cfg))
	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "benchmark-pod",
			"namespace": "default",
			"labels":    map[string]interface{}{"app": "benchmark"},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "registry.example.io/app:1.0.0",
					"securityContext": map[string]interface{}{
						"allowPrivilegeEscalation": false,
						"runAsNonRoot":             true,
					},
				},
				map[string]interface{}{
					"name":  "sidecar",
					"image": "registry.example.io/sidecar:1.0.0",
				},
			},
		},
	}
	if err := AddResource(ctx, mustJSON(pod)); err != nil {
		b.Fatal(err)
	}
	if err := ctx.AddServiceAccount("system:serviceaccount:default:benchmark"); err != nil {
		b.Fatal(err)
	}
	if err := ctx.AddOperation("CREATE"); err != nil {
		b.Fatal(err)
	}
	return ctx
}

func BenchmarkCheckpointRestore(b *testing.B) {
	ctx := benchmarkPodContext(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		ctx.Restore()
	}
}

func BenchmarkCheckpointReset(b *testing.B) {
	ctx := benchmarkPodContext(b)
	ctx.Checkpoint()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Reset()
	}
	ctx.Restore()
}

func BenchmarkCheckpointAddResourceRestore(b *testing.B) {
	ctx := benchmarkPodContext(b)
	altPod := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "benchmark-pod",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "registry.example.io/app:2.0.0",
				},
			},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx.Checkpoint()
		if err := ctx.AddResource(altPod); err != nil {
			b.Fatal(err)
		}
		ctx.Restore()
	}
}
