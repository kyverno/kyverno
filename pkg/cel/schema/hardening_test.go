package schema_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeDir writes resources to a temp dir and loads them.
func load(t *testing.T, yaml string, opts schema.Options) (*schema.Registry, error) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "r.yaml"), []byte(yaml), 0o600))
	return schema.LoadDir(dir, opts)
}

const schemaDoc = `apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadSchema
metadata: {name: %s}
spec:
  dialect: JSONSchema202012SubsetV1
  compatibilityVersion: "1"
  schema: %s
`

func schemaYAML(name, body string) string {
	return strings.Replace(strings.Replace(schemaDoc, "%s", name, 1), "%s", body, 1)
}

func TestProfileRejectsUnsupportedKeywords(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"patternProperties":  `{type: object, patternProperties: {"^x": {type: string}}}`,
		"remote ref":         `{type: object, properties: {a: {$ref: "https://example.com/s.json"}}}`,
		"conditional":        `{type: object, if: {required: [a]}}`,
		"nested prefixItems": `{type: object, properties: {a: {type: array, prefixItems: [{type: string}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := load(t, schemaYAML("s", body), schema.Options{})
			assert.ErrorContains(t, err, "PayloadSchema s")
		})
	}
	// A property merely *named* like a keyword is fine.
	_, err := load(t, schemaYAML("s", `{type: object, properties: {if: {type: string}}}`), schema.Options{})
	assert.NoError(t, err)
	_, err = load(t, strings.Replace(schemaYAML("s", `{type: object}`), "JSONSchema202012SubsetV1", "Other", 1), schema.Options{})
	assert.ErrorContains(t, err, "dialect")
}

func TestDigest(t *testing.T) {
	t.Parallel()
	a, err := load(t, schemaYAML("s", `{type: object, properties: {a: {type: string}}}`), schema.Options{})
	require.NoError(t, err)
	same, err := load(t, schemaYAML("s", `{properties: {a: {type: string}}, type: object}`), schema.Options{})
	require.NoError(t, err)
	changed, err := load(t, schemaYAML("s", `{type: object, properties: {a: {type: integer}}}`), schema.Options{})
	require.NoError(t, err)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, a.SchemaDigest("s"))
	assert.Equal(t, a.SchemaDigest("s"), same.SchemaDigest("s"), "key order must not matter")
	assert.NotEqual(t, a.SchemaDigest("s"), changed.SchemaDigest("s"))
}

func TestExposureProjection(t *testing.T) {
	t.Parallel()
	s, err := schema.New([]byte(`{"type":"object","properties":{
	  "public":{"type":"string"},
	  "secret":{"type":"string"},
	  "nested":{"type":"object","properties":{"keep":{"type":"string"},"drop":{"type":"string"}}}}}`), nil)
	require.NoError(t, err)
	s.WithExposure(schema.Exposure{Include: []string{"/**"}, Exclude: []string{"/secret", "/nested/drop"}})
	p, err := s.Decode([]byte(`{"public":"p","secret":"s","nested":{"keep":"k","drop":"d"}}`))
	require.NoError(t, err)
	for expr, want := range map[string]any{
		`has(object.public)`:        true,
		`has(object.secret)`:        false,
		`has(object.nested.keep)`:   true,
		`has(object.nested.drop)`:   false,
		`object.nested.keep == 'k'`: true,
	} {
		got, err := s.Eval(expr, p)
		require.NoError(t, err, expr)
		assert.Equal(t, want, got, expr)
	}
	_, err = s.Eval(`object.secret == 's'`, p)
	assert.Error(t, err, "excluded field must not be readable")

	only, err := schema.New([]byte(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}}}`), nil)
	require.NoError(t, err)
	only.WithExposure(schema.Exposure{Include: []string{"/a"}})
	p, err = only.Decode([]byte(`{"a":"1","b":"2"}`))
	require.NoError(t, err)
	got, _ := only.Eval(`has(object.b)`, p)
	assert.Equal(t, false, got)
}

const policyAndBinding = `
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: %s}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validationActions: [%s]
  validations:
    - expression: "false"
      message: nope
---
apiVersion: policies.kyverno.io/v1alpha1
kind: PayloadBinding
metadata: {name: b}
spec:
  route: {path: /x, methods: [POST]}
  %s
  selection: {schemaRef: {name: s}}
  %s
  %s
`

func bindingYAML(actions, auth, required, extra string) string {
	s := schemaYAML("s", `{type: object}`) + policyAndBinding
	for _, v := range []string{"p", actions, auth, required, extra} {
		s = strings.Replace(s, "%s", v, 1)
	}
	return s
}

func TestBindingReadiness(t *testing.T) {
	t.Parallel()
	req := "requiredPolicies: [{name: p}]"
	_, err := load(t, bindingYAML("Deny", "", req, ""), schema.Options{})
	assert.NoError(t, err)
	_, err = load(t, bindingYAML("Deny", "", "", ""), schema.Options{})
	assert.ErrorContains(t, err, "no required enforcing policy")
	_, err = load(t, bindingYAML("Audit", "", req, ""), schema.Options{})
	assert.ErrorContains(t, err, "no Deny action")
	_, err = load(t, bindingYAML("Deny", "", "requiredPolicies: [{name: missing}]", ""), schema.Options{})
	assert.ErrorContains(t, err, "unknown required policy")
}

func TestEventsMissingCandidateCoverage(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("examples", "events.yaml"))
	require.NoError(t, err)
	broken := strings.Replace(string(b), "    - name: prod-deploy-needs-approval\n  enforcement", "  enforcement", 1)
	require.NotEqual(t, string(b), broken)
	_, err = load(t, broken, schema.Options{})
	assert.ErrorContains(t, err, `schema "deploy-event-v1" has no required enforcing policy`)
}

func TestAuditOnlyPolicyDoesNotBlock(t *testing.T) {
	t.Parallel()
	y := bindingYAML("Deny", "", "requiredPolicies: [{name: p}]", "") + `
---
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata: {name: audit-only}
spec:
  evaluation: {mode: Schema, schemaRef: {name: s}}
  validationActions: [Audit]
  validations:
    - expression: "false"
`
	reg, err := load(t, y, schema.Options{})
	require.NoError(t, err)
	srv := httptest.NewServer(reg)
	defer srv.Close()
	_, out := post(t, srv, "/x", `{}`)
	assert.Equal(t, "denied", out["decision"])
	assert.Equal(t, "p: nope", out["reason"], "only the enforcing policy appears in the denial")
}

func TestLimits(t *testing.T) {
	t.Parallel()
	req := "requiredPolicies: [{name: p}]"
	reg, err := load(t, bindingYAML("Deny", "", req, "limits: {maxBodyBytes: 64, maxDepth: 3}"), schema.Options{})
	require.NoError(t, err)
	srv := httptest.NewServer(reg)
	defer srv.Close()
	code, _ := post(t, srv, "/x", `{"pad":"`+strings.Repeat("a", 200)+`"}`)
	assert.Equal(t, http.StatusRequestEntityTooLarge, code)
	_, out := post(t, srv, "/x", `{"a":{"b":{"c":{}}}}`)
	assert.Contains(t, out["reason"], "maxDepth")

	_, err = load(t, bindingYAML("Deny", "", req, "limits: {maxCandidates: 1}")+"", schema.Options{})
	assert.NoError(t, err, "single schema binding has no candidates")
	b, _ := os.ReadFile(filepath.Join("examples", "events.yaml"))
	_, err = load(t, strings.Replace(string(b), "maxCandidates: 4", "maxCandidates: 1", 1), schema.Options{})
	assert.ErrorContains(t, err, "maxCandidates")
}

// --- mTLS authorization ---

type pki struct {
	caPEM  []byte
	ca     *x509.Certificate
	caKey  *ecdsa.PrivateKey
	serial int64
}

func newPKI(t *testing.T) *pki {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &pki{ca: ca, caKey: key, serial: 1, caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func (p *pki) issue(t *testing.T, tmpl *x509.Certificate) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	p.serial++
	tmpl.SerialNumber = big.NewInt(p.serial)
	tmpl.NotBefore, tmpl.NotAfter = time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, p.ca, &key.PublicKey, p.caKey)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func (p *pki) client(t *testing.T, spiffe string) *tls.Certificate {
	u, err := url.Parse(spiffe)
	require.NoError(t, err)
	c := p.issue(t, &x509.Certificate{
		Subject: pkix.Name{CommonName: "client"}, URIs: []*url.URL{u},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	return &c
}

func TestAuthorization(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	reg, err := schema.LoadDir("examples", schema.Options{Principal: schema.MTLSPrincipal})
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(p.ca)
	serverCert := p.issue(t, &x509.Certificate{
		Subject: pkix.Name{CommonName: "server"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	srv := httptest.NewUnstartedServer(reg)
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.VerifyClientCertIfGiven, ClientCAs: pool}
	srv.StartTLS()
	defer srv.Close()

	do := func(cert *tls.Certificate, path, body string) int {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
		if cert != nil {
			cfg.Certificates = []tls.Certificate{*cert}
		}
		c := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
		resp, err := c.Post(srv.URL+path, "application/json", strings.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}
	const nono = `{"backend":"k","request":{"capability_type":"endpoint","request_id":"r","session_id":"s"}}`
	agent := p.client(t, "spiffe://example.com/nono-agent")
	builder := p.client(t, "spiffe://example.com/build-system")

	assert.Equal(t, 200, do(agent, "/approve", nono), "allowed principal")
	assert.Equal(t, 401, do(nil, "/approve", nono), "no client certificate")
	assert.Equal(t, 403, do(builder, "/approve", nono), "known route, principal not allowed on this binding")
	assert.Equal(t, 403, do(agent, "/events", `{}`), "no cross-binding fallback")
	assert.Equal(t, 404, do(agent, "/unknown", `{}`), "unknown route")

	// A client cert from an untrusted CA never reaches the handler.
	rogue := newPKI(t).client(t, "spiffe://example.com/nono-agent")
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool, Certificates: []tls.Certificate{*rogue}}
	_, err = (&http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}).Post(srv.URL+"/approve", "application/json", strings.NewReader(nono))
	assert.Error(t, err)
}

func TestAuthorizationReadiness(t *testing.T) {
	t.Parallel()
	opts := schema.Options{Principal: schema.MTLSPrincipal}
	req := "requiredPolicies: [{name: p}]"
	_, err := load(t, bindingYAML("Deny", "", req, ""), opts)
	assert.ErrorContains(t, err, "allowedPrincipals must not be empty")
	_, err = load(t, bindingYAML("Deny", "authorization: {allowedPrincipals: [a]}", req, "enforcement: {requireApplicablePolicy: false}"), opts)
	assert.ErrorContains(t, err, "requireApplicablePolicy must stay true")
	_, err = load(t, bindingYAML("Deny", "authorization: {allowedPrincipals: [a]}", req, ""), opts)
	assert.NoError(t, err)
}
