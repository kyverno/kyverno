package updaterequest

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
)

// TestNewFake verifies that a fake generator is created successfully
func TestNewFake(t *testing.T) {
	gen := NewFake()

	assert.NotNil(t, gen)
}

func TestFakeGenerator_Apply(t *testing.T) {
	gen := NewFake()

	spec := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: "test-policy",
	}

	err := gen.Apply(context.Background(), spec)

	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_GenerateType(t *testing.T) {
	gen := NewFake()

	spec := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Generate,
		Policy: "test-policy",
	}

	err := gen.Apply(context.Background(), spec)

	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_EmptySpec(t *testing.T) {
	gen := NewFake()

	spec := kyvernov2.UpdateRequestSpec{}

	err := gen.Apply(context.Background(), spec)

	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_WithContext(t *testing.T) {
	gen := NewFake()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spec := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: "test-policy",
	}

	err := gen.Apply(ctx, spec)

	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_CancelledContext(t *testing.T) {
	gen := NewFake()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	spec := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: "test-policy",
	}

	err := gen.Apply(ctx, spec)

	// Fake generator should still succeed even with cancelled context
	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name string
		spec kyvernov2.UpdateRequestSpec
	}{
		{
			name: "mutate type",
			spec: kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.Mutate,
				Policy: "test-mutate-policy",
			},
		},
		{
			name: "generate type",
			spec: kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.Generate,
				Policy: "test-generate-policy",
			},
		},
		{
			name: "empty policy name",
			spec: kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.Mutate,
				Policy: "",
			},
		},
		{
			name: "with rule context",
			spec: kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.Generate,
				Policy: "test-policy",
				RuleContext: []kyvernov2.RuleContext{
					{
						Rule: "test-rule",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewFake()

			err := gen.Apply(context.Background(), tt.spec)

			assert.NoError(t, err)
		})
	}
}

func TestFakeGenerator_ImplementsGeneratorInterface(t *testing.T) {
	var _ Generator = NewFake()

	// If this compiles, the interface is satisfied
}

func TestFakeGenerator_Apply_WithResource(t *testing.T) {
	gen := NewFake()

	spec := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: "test-policy",
		Resource: kyvernov1.ResourceSpec{
			Kind:       "Pod",
			APIVersion: "v1",
			Namespace:  "default",
			Name:       "test-pod",
		},
	}

	err := gen.Apply(context.Background(), spec)

	assert.NoError(t, err)
}

func TestFakeGenerator_Apply_Concurrent(t *testing.T) {
	gen := NewFake()

	// Run multiple applies concurrently
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			spec := kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.Mutate,
				Policy: "test-policy",
			}
			errChan <- gen.Apply(context.Background(), spec)
		}(i)
	}

	// Wait for all goroutines and check errors
	for i := 0; i < 10; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}
}
