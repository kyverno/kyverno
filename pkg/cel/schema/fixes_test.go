package schema_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExposureEmptyIncludeExposesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := []byte(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	omitted, err := schema.New(raw, nil)
	require.NoError(t, err)
	omitted.WithExposure(schema.Exposure{})
	p, err := omitted.Decode(ctx, []byte(`{"a":"x"}`))
	require.NoError(t, err)
	got, _ := omitted.Evaluate(ctx, `has(object.a)`, p)
	assert.Equal(t, true, got, "no include list means everything")

	empty, err := schema.New(raw, nil)
	require.NoError(t, err)
	empty.WithExposure(schema.Exposure{Include: []string{}})
	p, err = empty.Decode(ctx, []byte(`{"a":"x"}`))
	require.NoError(t, err)
	got, _ = empty.Evaluate(ctx, `has(object.a)`, p)
	assert.Equal(t, false, got, "an explicitly empty include list exposes nothing")
}

func TestExposureWildcards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := schema.New([]byte(`{"type":"object","properties":{
	  "commands":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"argv":{"type":"array","items":{"type":"string"}}}}},
	  "headers":{"type":"object","additionalProperties":{"type":"string"}}}}`), nil)
	require.NoError(t, err)
	s.WithExposure(schema.Exposure{Exclude: []string{"/commands/*/argv", "/headers/*"}})
	p, err := s.Decode(ctx, []byte(`{"commands":[{"name":"a","argv":["x"]},{"name":"b","argv":["y"]}],"headers":{"authorization":"secret"}}`))
	require.NoError(t, err)
	for expr, want := range map[string]any{
		`object.commands[0].name == 'a' && object.commands[1].name == 'b'`: true,
		`has(object.commands[0].argv) || has(object.commands[1].argv)`:     false,
		`'authorization' in object.headers`:                                false,
		`size(object.headers) == 0`:                                        true,
	} {
		got, err := s.Evaluate(ctx, expr, p)
		require.NoError(t, err, expr)
		assert.Equal(t, want, got, expr)
	}
}

const routeBase = `
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: p}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validations:
    - expression: "true"
`

func binding(name, methods, path, extra string) string {
	return `---
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadBinding
metadata: {name: ` + name + `}
spec:
  route: {path: ` + path + `, methods: ` + methods + `}
  selection: {schemaRef: {name: s}}
  requiredPolicies: [{name: p}]
` + extra
}

func TestRoutesAreKeyedByMethodAndPath(t *testing.T) {
	t.Parallel()
	base := schemaYAML("s", `{type: object}`) + routeBase
	reg, err := load(t, base+binding("get", "[GET]", "/check", "")+binding("post", "[POST]", "/check", ""), schema.Options{})
	require.NoError(t, err, "disjoint methods on one path do not conflict")
	srv := httptest.NewServer(reg)
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/check", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, srv.URL+"/check", http.NoBody)
	require.NoError(t, err)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	_, err = load(t, base+binding("a", "[POST]", "/check", "")+binding("b", "[GET, POST]", "/check", ""), schema.Options{})
	assert.ErrorContains(t, err, "already registered")
}

func TestBindingValidation(t *testing.T) {
	t.Parallel()
	base := schemaYAML("s", `{type: object}`) + routeBase
	for name, tc := range map[string]struct{ extra, want string }{
		"status too small":    {"  response: {statusCode: 99}", "not a valid HTTP status"},
		"status too large":    {"  response: {statusCode: 1000}", "not a valid HTTP status"},
		"maxCELCost":          {"  limits: {maxCELCost: 10}", "maxCELCost is not supported"},
		"template ignores it": {"  response: {bodyExpression: '{\"result\": \"ok\"}'}", "must depend on decision.allowed"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := load(t, base+binding("b", "[POST]", "/x", tc.extra), schema.Options{})
			assert.ErrorContains(t, err, tc.want)
		})
	}
	_, err := load(t, base+binding("b", "[POST]", "/x", "  response: {statusCode: 202}"), schema.Options{})
	assert.NoError(t, err)
}

func TestBareDynBooleanIsAccepted(t *testing.T) {
	t.Parallel()
	y := schemaYAML("s", `{type: object}`) + `
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: p}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validations:
    - expression: "transport.method == 'POST'"
`
	_, err := load(t, y, schema.Options{})
	assert.NoError(t, err, "dyn-typed expressions can return a bool and must not be rejected")
}

// A rejected request gets the fixed denial. The template never sees it.
func TestRejectedRequestSkipsTemplate(t *testing.T) {
	t.Parallel()
	base := schemaYAML("s", `{type: object, required: [a], properties: {a: {type: string}}}`) + routeBase
	reg, err := load(t, base+binding("b", "[POST]", "/x", "  response: {bodyExpression: 'decision.allowed ? {\"result\": \"allow\"} : {\"result\": \"deny\"}'}"), schema.Options{})
	require.NoError(t, err)
	srv := httptest.NewServer(reg)
	defer srv.Close()
	_, out := post(t, srv, "/x", `{"a":"ok"}`)
	assert.Equal(t, "allow", out["result"])
	_, out = post(t, srv, "/x", `{}`)
	assert.Equal(t, "denied", out["decision"], "validation failure uses the fixed body")
	assert.NotContains(t, out, "result")
	_, out = post(t, srv, "/x", `{"a":"x","a":"y"}`)
	assert.Equal(t, "denied", out["decision"])
	assert.Contains(t, out["reason"], "duplicate object key")
}

// An Audit policy is observational: whatever it does, it must never change the
// decision the enforcing policy reached, in either direction.
func TestAuditPolicyNeverChangesTheDecision(t *testing.T) {
	t.Parallel()
	const (
		pass      = `"true"`
		fail      = `"false"`
		evalError = `"object.a == 'x'"` // the payload has no "a", so this errors at runtime
	)
	tests := []struct {
		name, enforcing, audit string
		decision, reason       string
	}{
		{"enforcing passes, audit passes", pass, pass, "granted", ""},
		{"enforcing passes, audit fails", pass, fail, "granted", ""},
		{"enforcing passes, audit errors", pass, evalError, "granted", ""},
		{"enforcing fails, audit passes", fail, pass, "denied", "enf: nope"},
		{"enforcing fails, audit fails", fail, fail, "denied", "enf: nope"},
		{"enforcing fails, audit errors", fail, evalError, "denied", "enf: nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			y := schemaYAML("s", `{type: object, properties: {a: {type: string}}}`) + `
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: enf}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validations:
    - expression: ` + tc.enforcing + `
      message: nope
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: audit-only}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validationActions: [Audit]
  validations:
    - expression: ` + tc.audit + `
      message: audit says no
` + binding("b", "[POST]", "/x", "")
			y = strings.Replace(y, "requiredPolicies: [{name: p}]", "requiredPolicies: [{name: enf}]", 1)
			reg, err := load(t, y, schema.Options{})
			require.NoError(t, err)
			srv := httptest.NewServer(reg)
			defer srv.Close()
			_, out := post(t, srv, "/x", `{}`)
			assert.Equal(t, tc.decision, out["decision"])
			if tc.reason == "" {
				assert.NotContains(t, out, "reason")
			} else {
				assert.Equal(t, tc.reason, out["reason"], "the audit policy's message must not leak into the denial")
			}
		})
	}
}
