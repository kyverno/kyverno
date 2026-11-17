package schema_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "nono", name))
	require.NoError(t, err)
	return b
}

func nonoSchema(t *testing.T) *schema.Schema {
	t.Helper()
	var computed []schema.Computed
	require.NoError(t, json.Unmarshal(read(t, "computed.json"), &computed))
	s, err := schema.New(read(t, "schema.json"), computed)
	require.NoError(t, err)
	return s
}

const readOnlyKubectl = `object.request.capability_type != 'command' ||
  object.request.command != 'kubectl' || (derived.shimOk && !derived.isMutatingKubectl)`

func TestNonoDecision(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := nonoSchema(t)
	tests := []struct {
		fixture string
		allowed bool
	}{
		{"kubectl-get.json", true},
		{"kubectl-delete-ns.json", false},
		// args[0] is not the shim, so "delete" must not be mistaken for it.
		{"kubectl-delete-unprefixed.json", false},
		{"endpoint.json", true},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()
			p, err := s.Decode(ctx, read(t, tc.fixture))
			require.NoError(t, err)
			got, err := s.Evaluate(ctx, readOnlyKubectl, p)
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, got)
		})
	}
}

func TestNonoRejectedPayloads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := nonoSchema(t)
	for _, f := range []string{"invalid-missing-request-id.json", "invalid-pid-type.json"} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			_, err := s.Decode(ctx, read(t, f))
			assert.Error(t, err)
		})
	}
	for name, body := range map[string]string{
		"trailing document":        `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","method":"GET","path":"/"}} {}`,
		"endpoint without path":    `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","method":"GET"}}`,
		"endpoint without method":  `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","path":"/"}}`,
		"command without command":  `{"backend":"k","request":{"capability_type":"command","request_id":"r","session_id":"s"}}`,
		"duplicate key":            `{"backend":"k","backend":"other","request":{"capability_type":"command","request_id":"r","session_id":"s","command":"ls"}}`,
		"duplicate nested key":     `{"backend":"k","request":{"capability_type":"command","capability_type":"endpoint","request_id":"r","session_id":"s","command":"ls"}}`,
		"capability not in enum":   `{"backend":"k","request":{"capability_type":"other","request_id":"r","session_id":"s"}}`,
		"command and wrong method": `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","command":"ls"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := s.Decode(ctx, []byte(body))
			assert.Error(t, err)
		})
	}
}

// Typed fields: the compiler should reject unknown or mistyped declared fields.
func TestStaticTypeChecking(t *testing.T) {
	t.Parallel()
	s := nonoSchema(t)
	assert.NoError(t, s.Check(`object.request.command == 'kubectl'`))
	assert.Error(t, s.Check(`object.request.command == 5`), "string vs int")
	assert.Error(t, s.Check(`object.request.nonexistent == 'x'`), "undeclared field")
	assert.Error(t, s.Check(`object.request.child_pid == 'x'`), "int vs string")
}

// Presence: optional fields absent from the payload.
func TestPresence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := nonoSchema(t)
	p, err := s.Decode(ctx, read(t, "endpoint.json"))
	require.NoError(t, err)
	got, err := s.Evaluate(ctx, `has(object.request.command)`, p)
	require.NoError(t, err)
	assert.Equal(t, false, got)
	// Accessing an absent optional field must error, not yield a zero value.
	_, err = s.Evaluate(ctx, `object.request.command == ''`, p)
	assert.Error(t, err)
}

// 1.0 and 1e0 are integers under JSON Schema, so an integer field must accept them.
func TestIntegralFloatIsAnInteger(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := nonoSchema(t)
	for _, v := range []string{"1.0", "1e0", "10e-1"} {
		p, err := s.Decode(ctx, []byte(`{"backend":"k","request":{"capability_type":"command","request_id":"r","session_id":"s","command":"ls","child_pid":`+v+`}}`))
		require.NoError(t, err, v)
		got, err := s.Evaluate(ctx, `object.request.child_pid == 1`, p)
		require.NoError(t, err)
		assert.Equal(t, true, got, v)
	}
	_, err := s.Decode(ctx, []byte(`{"backend":"k","request":{"capability_type":"command","request_id":"r","session_id":"s","command":"ls","child_pid":1.5}}`))
	assert.Error(t, err, "a fractional value is not an integer")
}

// Numbers follow the declared type, not the value: 5 for a `number` field is a double.
func TestNumbersFollowTheSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := schema.New([]byte(`{"type":"object","properties":{
	  "price":{"type":"number"},
	  "count":{"type":"integer"},
	  "items":{"type":"array","items":{"type":"number"}},
	  "free":{"type":"object","additionalProperties":{"type":"integer"}}}}`), nil)
	require.NoError(t, err)
	p, err := s.Decode(ctx, []byte(`{"price":5,"count":2,"items":[1,2.5],"free":{"a":3}}`))
	require.NoError(t, err)
	for expr, want := range map[string]any{
		`object.price + 1.5 == 6.5`:    true, // double arithmetic, though the JSON text was "5"
		`object.count + 1 == 3`:        true,
		`object.items[0] + 0.5 == 1.5`: true,
		`object.free['a'] + 1 == 4`:    true,
	} {
		got, err := s.Evaluate(ctx, expr, p)
		require.NoError(t, err, expr)
		assert.Equal(t, want, got, expr)
	}
}

func TestParseJSONLimits(t *testing.T) {
	t.Parallel()
	_, err := schema.ParseJSON([]byte(`{"a":1,"a":2}`), 8)
	assert.ErrorContains(t, err, "duplicate object key")
	_, err = schema.ParseJSON([]byte(`{"a":{"b":{"c":1}}}`), 2)
	assert.ErrorContains(t, err, "maxDepth")
	_, err = schema.ParseJSON([]byte(`{"a":{"b":{"c":1}}}`), 3)
	assert.NoError(t, err)
	_, err = schema.ParseJSON([]byte(`{"a":1} {"b":2}`), 8)
	assert.ErrorContains(t, err, "trailing")
	// Same key in sibling objects is fine.
	_, err = schema.ParseJSON([]byte(`[{"a":1},{"a":2}]`), 8)
	assert.NoError(t, err)
	// A hostile, deeply nested body is refused without recursing to the bottom.
	deep := make([]byte, 0, 2_000_000)
	for i := 0; i < 1_000_000; i++ {
		deep = append(deep, '[')
	}
	_, err = schema.ParseJSON(deep, 64)
	assert.ErrorContains(t, err, "maxDepth")
}
