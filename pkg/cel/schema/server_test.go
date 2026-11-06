package schema_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	reg, err := schema.LoadDir("examples", schema.Options{})
	require.NoError(t, err)
	srv := httptest.NewServer(reg)
	t.Cleanup(srv.Close)
	return srv
}

func post(t *testing.T, srv *httptest.Server, path, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestApproveNono(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tests := map[string]string{
		"kubectl-get.json":                "granted",
		"kubectl-delete-ns.json":          "denied",
		"endpoint.json":                   "granted",
		"invalid-missing-request-id.json": "denied",
		"invalid-pid-type.json":           "denied",
	}
	for fixture, want := range tests {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(filepath.Join("testdata", "nono", fixture))
			require.NoError(t, err)
			code, out := post(t, srv, "/approve", string(body))
			assert.Equal(t, 200, code)
			assert.Equal(t, want, out["decision"])
		})
	}
}

// Two policies apply to the nono schema; either one denying denies (deny-overrides).
func TestApproveSecondPolicyDenies(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	_, out := post(t, srv, "/approve", `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s","method":"GET","path":"/api/secrets/x"}}`)
	assert.Equal(t, "denied", out["decision"])
	assert.Contains(t, out["reason"], "nono-no-secrets-endpoint")
}

func TestEventsSelection(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	tests := []struct {
		name, body, schema, reason string
		allowed                    bool
	}{
		{"build ok", `{"eventType":"build.completed","repository":"r","artifacts":[{"name":"a.tar","size":10}]}`, "build-event-v1", "", true},
		{"build too big", `{"eventType":"build.completed","repository":"r","artifacts":[{"name":"a.tar","size":209715200}]}`, "build-event-v1", "exceeds 100MiB", false},
		{"build exe", `{"eventType":"build.completed","repository":"r","artifacts":[{"name":"a.exe","size":10}]}`, "build-event-v1", "executables", false},
		{"deploy dev ok", `{"eventType":"deploy.requested","environment":"dev","image":"r/app@sha256:abc"}`, "deploy-event-v1", "", true},
		{"deploy prod unapproved", `{"eventType":"deploy.requested","environment":"prod","image":"r/app@sha256:abc"}`, "deploy-event-v1", "approvedBy", false},
		{"deploy two violations", `{"eventType":"deploy.requested","environment":"prod","image":"r/app:latest"}`, "deploy-event-v1", "digest", false},
		{"deploy prod approved", `{"eventType":"deploy.requested","environment":"prod","image":"r/app@sha256:abc","approvedBy":"alice"}`, "deploy-event-v1", "", true},
		{"unknown event", `{"eventType":"other"}`, "", "no candidate", false},
		// Selected by discriminator, then rejected by validation; no retry on another schema.
		{"build with extra field", `{"eventType":"build.completed","repository":"r","artifacts":[],"extra":1}`, "build-event-v1", "does not match schema", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, out := post(t, srv, "/events", tc.body)
			assert.Equal(t, tc.allowed, out["allowed"])
			if tc.schema != "" {
				assert.Equal(t, tc.schema, out["schema"])
			}
			if tc.reason != "" {
				assert.Contains(t, out["reason"], tc.reason)
			}
		})
	}
}

func TestProtocolErrors(t *testing.T) {
	t.Parallel()
	srv := newServer(t)
	resp, err := http.Get(srv.URL + "/approve")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	resp, err = http.Post(srv.URL+"/nope", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_, out := post(t, srv, "/approve", "not json")
	assert.Equal(t, "denied", out["decision"])
}

func TestLoadRejectsBadResources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(s string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "r.yaml"), []byte(s), 0o600))
	}
	base := `apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata: {name: s}
spec:
  dialect: JSONSchema202012SubsetV1
  schema: {type: object, properties: {a: {type: string}}}
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: p}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validations:
    - expression: `
	write(base + "object.a == 'x'")
	_, err := schema.LoadDir(dir, schema.Options{})
	assert.NoError(t, err)
	write(base + "object.nope == 'x'")
	_, err = schema.LoadDir(dir, schema.Options{})
	assert.ErrorContains(t, err, "ValidatingPolicy p")
	write(base + "object.a")
	_, err = schema.LoadDir(dir, schema.Options{})
	assert.ErrorContains(t, err, "must return bool")
}
