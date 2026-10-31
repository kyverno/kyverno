package schema_test

import (
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
  object.request.command != 'kubectl' || !derived.isMutatingKubectl`

func TestNonoDecision(t *testing.T) {
	t.Parallel()
	s := nonoSchema(t)
	tests := []struct {
		fixture string
		allowed bool
	}{
		{"kubectl-get.json", true},
		{"kubectl-delete-ns.json", false},
		{"endpoint.json", true},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()
			p, err := s.Decode(read(t, tc.fixture))
			require.NoError(t, err)
			got, err := s.Eval(readOnlyKubectl, p)
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, got)
		})
	}
}

func TestNonoRejectedPayloads(t *testing.T) {
	t.Parallel()
	s := nonoSchema(t)
	for _, f := range []string{"invalid-missing-request-id.json", "invalid-pid-type.json"} {
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			_, err := s.Decode(read(t, f))
			assert.Error(t, err)
		})
	}
	_, err := s.Decode([]byte(`{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s"}} {}`))
	assert.Error(t, err, "trailing document must be rejected")
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
	s := nonoSchema(t)
	p, err := s.Decode(read(t, "endpoint.json"))
	require.NoError(t, err)
	got, err := s.Eval(`has(object.request.command)`, p)
	require.NoError(t, err)
	assert.Equal(t, false, got)
	// Accessing an absent optional field must error, not yield a zero value.
	_, err = s.Eval(`object.request.command == ''`, p)
	assert.Error(t, err)
}

// Numbers: 1.0 is an integer under JSON Schema (proposal, numeric section), but
// the kube-openapi validator rejects it when handed a json.Number. Pinned as a
// known gap (see FINDINGS.md); flip to NoError once a conforming validator or
// schema-driven normalization lands.
func TestIntegralFloatPidKnownGap(t *testing.T) {
	t.Parallel()
	s := nonoSchema(t)
	_, err := s.Decode([]byte(`{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","child_pid":1.0}}`))
	assert.Error(t, err)
}
