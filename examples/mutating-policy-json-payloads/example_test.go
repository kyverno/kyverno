package main

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// fakeContext implements libs.Context for testing
type fakeContext struct{}

func (f *fakeContext) APIResolver() (libs.APIResolver, error)        { return nil, nil }
func (f *fakeContext) RuntimeContextResolvers() []libs.ContextResolver { return nil }

func TestJSONPayloadExample(t *testing.T) {
	t.Run("process JSON payload with MutatingPolicy", func(t *testing.T) {
		// Create a sample JSON payload
		jsonPayload := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"user": map[string]interface{}{
					"name":  "john",
					"email": "john@example.com",
					"role":  "user",
				},
				"settings": map[string]interface{}{
					"theme":    "light",
					"language": "en",
				},
			},
		}

		// Create engine request from JSON payload
		request := engine.RequestFromJSON(&fakeContext{}, jsonPayload)

		// Create engine with no policies for this example
		pols := []policiesv1alpha1.MutatingPolicy{}
		polexs := []*policiesv1alpha1.PolicyException{}
		
		provider, err := mpolengine.NewProvider(compiler.NewCompiler(), pols, polexs)
		assert.NoError(t, err)

		eng := mpolengine.NewEngine(
			provider,
			func(string) *corev1.Namespace { return nil }, // namespace resolver
			nil, // matcher
			nil, // type converter
			&fakeContext{},
		)

		// Process the JSON payload
		response, err := eng.Handle(context.Background(), request, nil)

		// Verify the results
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.Resource)
		assert.Equal(t, jsonPayload, response.Resource)
		
		// Verify the original JSON payload is preserved
		user, found, err := unstructured.NestedMap(response.Resource.Object, "user")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, "john", user["name"])
		assert.Equal(t, "john@example.com", user["email"])
	})

	t.Run("MatchedMutateExistingPolicies with JSON payload", func(t *testing.T) {
		// Create a sample JSON payload
		jsonPayload := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"config": map[string]interface{}{
					"version": "1.0",
					"enabled": true,
				},
			},
		}

		// Create engine request from JSON payload
		request := engine.RequestFromJSON(&fakeContext{}, jsonPayload)

		// Create engine
		pols := []policiesv1alpha1.MutatingPolicy{}
		polexs := []*policiesv1alpha1.PolicyException{}
		
		provider, err := mpolengine.NewProvider(compiler.NewCompiler(), pols, polexs)
		assert.NoError(t, err)

		eng := mpolengine.NewEngine(
			provider,
			func(string) *corev1.Namespace { return nil },
			nil,
			nil,
			&fakeContext{},
		)

		// Test MatchedMutateExistingPolicies with JSON payload
		policies := eng.MatchedMutateExistingPolicies(context.Background(), request)

		// Should return an empty list since no policies are configured
		assert.NotNil(t, policies)
		assert.Len(t, policies, 0)
	})
}
