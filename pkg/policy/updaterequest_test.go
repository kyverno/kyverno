package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
)

func makeUR(n int) *kyvernov2.UpdateRequest {
	ur := &kyvernov2.UpdateRequest{}
	for i := 0; i < n; i++ {
		ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{
			Rule: "test-rule",
			Trigger: kyvernov1.ResourceSpec{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       "ns",
			},
		})
	}
	return ur
}

func TestSplitUR(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		batchSize     int
		wantBatches   int
		wantLastBatch int
	}{
		{"empty", 0, 100, 1, 0},
		{"below batch", 50, 100, 1, 50},
		{"exact batch", 100, 100, 1, 100},
		{"one over", 101, 100, 2, 1},
		{"10k namespaces", 10000, 100, 100, 100},
		{"uneven split", 250, 100, 3, 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ur := makeUR(tc.total)
			batches := splitUR(ur, tc.batchSize)
			assert.Len(t, batches, tc.wantBatches)
			assert.Len(t, batches[len(batches)-1].Spec.RuleContext, tc.wantLastBatch)

			// total entries across all batches must equal original
			total := 0
			for _, b := range batches {
				assert.LessOrEqual(t, len(b.Spec.RuleContext), tc.batchSize)
				total += len(b.Spec.RuleContext)
			}
			assert.Equal(t, tc.total, total)
		})
	}
}
