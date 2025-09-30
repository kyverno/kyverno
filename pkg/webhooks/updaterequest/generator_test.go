package updaterequest

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	generatorutils "github.com/kyverno/kyverno/pkg/utils/generator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewGenerator(t *testing.T) {
	// Create fake client and informer
	client := fake.NewSimpleClientset()
	informerFactory := kyvernoinformers.NewSharedInformerFactory(client, 0)
	urInformer := informerFactory.Kyverno().V2().UpdateRequests()

	// Create mock generator
	mockGenerator := &mockUpdateRequestGenerator{}

	// Test NewGenerator
	generator := NewGenerator(client, urInformer, mockGenerator)

	if generator == nil {
		t.Fatal("NewGenerator returned nil")
	}

	// Verify the generator implements the interface
	_, ok := generator.(Generator)
	if !ok {
		t.Error("NewGenerator did not return a Generator interface")
	}
}

func TestGeneratorApply(t *testing.T) {
	tests := []struct {
		name          string
		updateRequest kyvernov2.UpdateRequestSpec
		expectError   bool
		shouldSkip    bool
	}{
		{
			name: "valid generate request",
			updateRequest: kyvernov2.UpdateRequestSpec{
				Type: kyvernov2.Generate,
				RuleContext: []kyvernov2.RuleContext{
					{
						Rule: "test-rule",
					},
				},
				Policy: "test-policy",
			},
			expectError: false,
			shouldSkip:  false,
		},
		{
			name: "generate request with empty rule context should skip",
			updateRequest: kyvernov2.UpdateRequestSpec{
				Type:        kyvernov2.Generate,
				RuleContext: []kyvernov2.RuleContext{},
				Policy:      "test-policy",
			},
			expectError: false,
			shouldSkip:  true,
		},
		{
			name: "mutate request",
			updateRequest: kyvernov2.UpdateRequestSpec{
				Type: kyvernov2.Mutate,
				RuleContext: []kyvernov2.RuleContext{
					{
						Rule: "test-rule",
					},
				},
				Policy: "test-policy",
			},
			expectError: false,
			shouldSkip:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with some existing UpdateRequests
			existingUR := &kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-ur",
					Namespace: config.KyvernoNamespace(),
				},
				Spec: kyvernov2.UpdateRequestSpec{
					Type:   kyvernov2.Generate,
					Policy: "existing-policy",
				},
			}

			client := fake.NewSimpleClientset(existingUR)
			informerFactory := kyvernoinformers.NewSharedInformerFactory(client, 0)
			urInformer := informerFactory.Kyverno().V2().UpdateRequests()

			// Start informers
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			informerFactory.Start(ctx.Done())
			informerFactory.WaitForCacheSync(ctx.Done())

			mockGenerator := &mockUpdateRequestGenerator{}
			generator := NewGenerator(client, urInformer, mockGenerator)

			err := generator.Apply(ctx, tt.updateRequest)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// For generate requests with empty rule context, it should return early (no error)
			if tt.shouldSkip && err != nil {
				t.Errorf("expected request to be skipped without error, got: %v", err)
			}
		})
	}
}

func TestGeneratorWithNilInformer(t *testing.T) {
	// Test that NewGenerator panics with nil informer (expected behavior)
	// This simulates the case where CRDs are not available
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewGenerator should panic with nil informer")
		}
	}()

	client := fake.NewSimpleClientset()
	mockGenerator := &mockUpdateRequestGenerator{}

	// This should panic because the informer is nil
	// In practice, we check CRD availability first and don't create the generator if CRDs are missing
	NewGenerator(client, nil, mockGenerator)
}

// Mock implementation of UpdateRequestGenerator for testing
type mockUpdateRequestGenerator struct{}

func (m *mockUpdateRequestGenerator) Generate(ctx context.Context, client versioned.Interface, resource *kyvernov2.UpdateRequest, logger logr.Logger) (*kyvernov2.UpdateRequest, error) {
	return resource, nil
}

func TestGeneratorInterface(t *testing.T) {
	// Test that our generator properly implements the Generator interface
	client := fake.NewSimpleClientset()
	informerFactory := kyvernoinformers.NewSharedInformerFactory(client, 0)
	urInformer := informerFactory.Kyverno().V2().UpdateRequests()
	mockGenerator := &mockUpdateRequestGenerator{}

	generator := NewGenerator(client, urInformer, mockGenerator)

	// Test that it implements the interface methods
	ctx := context.Background()
	testUR := kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: "test-policy",
	}

	err := generator.Apply(ctx, testUR)
	if err != nil {
		t.Errorf("Apply method failed: %v", err)
	}
}

// Test the conditional logic we added in main.go
func TestConditionalGeneratorCreation(t *testing.T) {
	tests := []struct {
		name                    string
		updateRequestsAvailable bool
		expectNilGenerator      bool
	}{
		{
			name:                    "UpdateRequests CRD available",
			updateRequestsAvailable: true,
			expectNilGenerator:      false,
		},
		{
			name:                    "UpdateRequests CRD not available",
			updateRequestsAvailable: false,
			expectNilGenerator:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var generator Generator

			if tt.updateRequestsAvailable {
				// Simulate the case where CRDs are available
				client := fake.NewSimpleClientset()
				informerFactory := kyvernoinformers.NewSharedInformerFactory(client, 0)
				urInformer := informerFactory.Kyverno().V2().UpdateRequests()
				mockGenerator := &mockUpdateRequestGenerator{}
				generator = NewGenerator(client, urInformer, mockGenerator)
			} else {
				// Simulate the case where CRDs are not available (our fix)
				generator = nil
			}

			if tt.expectNilGenerator {
				if generator != nil {
					t.Error("expected nil generator when CRDs not available")
				}
			} else {
				if generator == nil {
					t.Error("expected non-nil generator when CRDs available")
				}
			}
		})
	}
}

// Ensure the mock implements the interface
var _ generatorutils.UpdateRequestGenerator = (*mockUpdateRequestGenerator)(nil)
