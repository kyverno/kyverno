package store

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

type mockJsonContext struct {
	enginecontext.Interface
	vars map[string]interface{}
}

func (m *mockJsonContext) AddVariable(key string, value interface{}) error {
	m.vars[key] = value
	return nil
}

func (m *mockJsonContext) Query(key string) (interface{}, error) {
	if val, ok := m.vars[key]; ok {
		return val, nil
	}
	return nil, nil
}

func TestContextLoaderFactory_Init(t *testing.T) {
	s := &Store{}
	s.SetLocal(true)
	s.SetPolicies(Policy{
		Name: "test-policy",
		Rules: []Rule{
			{
				Name: "test-rule",
				Values: map[string]interface{}{
					"foo": "bar",
				},
				ForEachValues: []map[string][]interface{}{
					{
						"baz": []interface{}{"qux", "quux"},
					},
					{
						"baz2": []interface{}{"qux2", "quux2"},
					},
				},
			},
		},
	})

	factory := ContextLoaderFactory(s, nil)

	policy := &kyvernov1.ClusterPolicy{}
	policy.SetName("test-policy")
	rule := kyvernov1.Rule{Name: "test-rule"}

	loader := factory(policy, rule)

	// Context with element 0, block 0
	ctx1 := &mockJsonContext{vars: map[string]interface{}{
		"foreachBlockIndex": int64(0),
		"elementIndex":      int64(0),
	}}

	err := loader.Load(context.Background(), nil, nil, nil, nil, ctx1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if val, ok := ctx1.vars["foo"]; !ok || val != "bar" {
		t.Errorf("Expected variable foo=bar, got %v", val)
	}
	if val, ok := ctx1.vars["baz"]; !ok || val != "qux" {
		t.Errorf("Expected variable baz=qux, got %v", val)
	}

	// Context with element 1, block 0
	ctx2 := &mockJsonContext{vars: map[string]interface{}{
		"foreachBlockIndex": int64(0),
		"elementIndex":      int64(1),
	}}
	if err := loader.Load(context.Background(), nil, nil, nil, nil, ctx2); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if val, ok := ctx2.vars["baz"]; !ok || val != "quux" {
		t.Errorf("Expected variable baz=quux, got %v", val)
	}

	// Context with out of bounds block
	ctx3 := &mockJsonContext{vars: map[string]interface{}{
		"foreachBlockIndex": int64(5),
		"elementIndex":      int64(0),
	}}
	if err := loader.Load(context.Background(), nil, nil, nil, nil, ctx3); err != nil {
		t.Fatalf("Unexpected error on out-of-bounds block: %v", err)
	}

	// Context with out of bounds element
	ctx4 := &mockJsonContext{vars: map[string]interface{}{
		"foreachBlockIndex": int64(0),
		"elementIndex":      int64(5),
	}}
	if err := loader.Load(context.Background(), nil, nil, nil, nil, ctx4); err != nil {
		t.Fatalf("Unexpected error on out-of-bounds element: %v", err)
	}
}
