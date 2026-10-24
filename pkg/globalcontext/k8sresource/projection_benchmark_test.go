package k8sresource

import (
	"fmt"
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// benchmarkCollectionQuery models a full-collection projection without
// measuring JMESPath parsing, informer scheduling, or the coalescing delay.
type benchmarkCollectionQuery struct{}

func (benchmarkCollectionQuery) Search(data interface{}) (interface{}, error) {
	count := 0
	for _, item := range data.([]interface{}) {
		if _, ok := item.(map[string]interface{})["metadata"]; ok {
			count++
		}
	}
	return count, nil
}

func newBenchmarkProjectionEntry(objects []runtime.Object) *entry {
	return &entry{
		lister:   &mockLister{objects: objects},
		gce:      &kyvernov2beta1.GlobalContextEntry{},
		eventGen: &mockEventGen{},
		projections: []store.Projection{
			{Name: "resource-count", JP: benchmarkCollectionQuery{}},
		},
		projected: make(map[string]interface{}),
	}
}

// BenchmarkProjectionRecomputations compares legacy per-event full scans
// against one full-snapshot refresh for the same synthetic workload.
// This isolates recomputation cost; it is not an end-to-end latency,
// full JMESPath, or production-performance benchmark.
//
// Run with -run '^$' -bench BenchmarkProjectionRecomputations -benchmem -count=5.
func BenchmarkProjectionRecomputations(b *testing.B) {
	for _, resourceCount := range []int{100, 1000} {
		objects := make([]runtime.Object, resourceCount)
		for i := range objects {
			objects[i] = &unstructured.Unstructured{Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": fmt.Sprintf("configmap-%d", i)},
			}}
		}

		b.Run(fmt.Sprintf("resources=%d/initial-list", resourceCount), func(b *testing.B) {
			for _, variant := range []struct {
				name   string
				perAdd bool
			}{{"legacy-per-add", true}, {"coalesced-snapshot", false}} {
				b.Run(variant.name, func(b *testing.B) {
					e := newBenchmarkProjectionEntry(objects)
					lister := e.lister.(*mockLister)
					b.ReportAllocs()
					b.ResetTimer()
					for iteration := 0; iteration < b.N; iteration++ {
						if variant.perAdd {
							for count := 1; count <= resourceCount; count++ {
								lister.objects = objects[:count]
								e.recomputeProjections()
							}
						} else {
							lister.objects = objects
							e.recomputeProjections()
						}
					}
				})
			}
		})

		const burstSize = 100
		b.Run(fmt.Sprintf("resources=%d/burst-of-%d", resourceCount, burstSize), func(b *testing.B) {
			for _, variant := range []struct {
				name           string
				recomputations int
			}{{"legacy-per-event", burstSize}, {"coalesced-snapshot", 1}} {
				b.Run(variant.name, func(b *testing.B) {
					e := newBenchmarkProjectionEntry(objects)
					b.ReportAllocs()
					b.ResetTimer()
					for iteration := 0; iteration < b.N; iteration++ {
						for refresh := 0; refresh < variant.recomputations; refresh++ {
							e.recomputeProjections()
						}
					}
				})
			}
		})
	}
}
