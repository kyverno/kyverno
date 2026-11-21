package processor

import (
	"os"
	"path/filepath"
	"testing"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestLoadHTTPRequests(t *testing.T) {
	validPath := filepath.Join("..", "..", "..", "..", "test", "cli", "test-validating-policy", "http-allow", "request.json")

	tests := []struct {
		name             string
		path             string
		content          string
		expectErr        bool
		wantErr          string
		checkErrContains bool
	}{
		{
			name: "valid http payload",
			path: validPath,
		},
		{
			name:             "missing file",
			path:             filepath.Join(t.TempDir(), "missing.json"),
			expectErr:        true,
			wantErr:          "failed to read HTTP payload file",
			checkErrContains: true,
		},
		{
			name:      "invalid json",
			path:      filepath.Join(t.TempDir(), "invalid-http.json"),
			content:   "{invalid-json",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.content != "" {
				require.NoError(t, os.WriteFile(tc.path, []byte(tc.content), 0o600))
			}

			req, err := LoadHTTPRequests(tc.path)
			if !tc.expectErr {
				require.NoError(t, err)
				require.NotNil(t, req)
				return
			}
			require.Error(t, err)
			if tc.checkErrContains {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestLoadEnvoyRequests(t *testing.T) {
	validPath := filepath.Join("..", "..", "..", "..", "test", "cli", "test-validating-policy", "envoy-allow", "request.json")

	tests := []struct {
		name             string
		path             string
		content          string
		expectErr        bool
		wantErr          string
		checkErrContains bool
	}{
		{
			name: "valid envoy payload",
			path: validPath,
		},
		{
			name:             "missing file",
			path:             filepath.Join(t.TempDir(), "missing.json"),
			expectErr:        true,
			wantErr:          "failed to read envoy payload file",
			checkErrContains: true,
		},
		{
			name:      "invalid proto json",
			path:      filepath.Join(t.TempDir(), "invalid-envoy.json"),
			content:   "{invalid-json",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.content != "" {
				require.NoError(t, os.WriteFile(tc.path, []byte(tc.content), 0o600))
			}

			req, err := LoadEnvoyRequests(tc.path)
			if !tc.expectErr {
				require.NoError(t, err)
				require.NotNil(t, req)
				return
			}
			require.Error(t, err)
			if tc.checkErrContains {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestAuthzProcessor_ApplyHTTPPolicies_EmptyInputs(t *testing.T) {
	processor := NewAuthzProcessor(&ResultCounts{}, nil, []*policiesv1beta1.ValidatingPolicy{}, nil, false)

	responses, err := processor.ApplyHTTPPolicies(nil)
	require.NoError(t, err)
	assert.Len(t, responses, 0)

	responses, err = processor.ApplyHTTPPolicies([]*authzhttp.CheckRequest{})
	require.NoError(t, err)
	assert.Len(t, responses, 0)
}

func TestAuthzProcessor_ApplyEnvoyPolicies_EmptyInputs(t *testing.T) {
	processor := NewAuthzProcessor(&ResultCounts{}, nil, nil, []*policiesv1beta1.ValidatingPolicy{}, false)

	responses, err := processor.ApplyEnvoyPolicies(nil)
	require.NoError(t, err)
	assert.Len(t, responses, 0)

	responses, err = processor.ApplyEnvoyPolicies([]*authv3.CheckRequest{})
	require.NoError(t, err)
	assert.Len(t, responses, 0)
}

// TestAuthzProcessor_ApplyHTTPPolicies_NonEmpty verifies that ApplyHTTPPolicies processes
// matching HTTP authorization policies and increments pass counts.
func TestAuthzProcessor_ApplyHTTPPolicies_NonEmpty(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "http-allow")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.HTTPPolicies, 1)

	req, err := LoadHTTPRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	rc := &ResultCounts{}
	processor := NewAuthzProcessor(rc, nil, res.HTTPPolicies, nil, false)
	responses, err := processor.ApplyHTTPPolicies([]*authzhttp.CheckRequest{req})
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.Equal(t, 1, rc.Pass)
}

// TestAuthzProcessor_ApplyHTTPPolicies_Deny verifies that ApplyHTTPPolicies handles
// denied HTTP authorization policies and honors the auditWarn configuration.
func TestAuthzProcessor_ApplyHTTPPolicies_Deny(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "http-deny")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.HTTPPolicies, 1)

	req, err := LoadHTTPRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	t.Run("auditWarn false with audit action counts failure as fail", func(t *testing.T) {
		p := *res.HTTPPolicies[0]
		p.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}

		rc := &ResultCounts{}
		processor := NewAuthzProcessor(rc, nil, []*policiesv1beta1.ValidatingPolicy{&p}, nil, false)
		responses, err := processor.ApplyHTTPPolicies([]*authzhttp.CheckRequest{req})
		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, 1, rc.Fail)
		assert.Equal(t, 0, rc.Warn)
		assert.Equal(t, 0, rc.Error)
	})

	t.Run("auditWarn true with audit action counts failure as warn", func(t *testing.T) {
		p := *res.HTTPPolicies[0]
		p.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}

		rc := &ResultCounts{}
		processor := NewAuthzProcessor(rc, nil, []*policiesv1beta1.ValidatingPolicy{&p}, nil, true)
		responses, err := processor.ApplyHTTPPolicies([]*authzhttp.CheckRequest{req})
		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, 0, rc.Fail)
		assert.Equal(t, 1, rc.Warn)
		assert.Equal(t, 0, rc.Error)
	})
}

// TestAuthzProcessor_ApplyEnvoyPolicies_NonEmpty verifies that ApplyEnvoyPolicies processes
// matching Envoy authorization policies and increments pass counts.
func TestAuthzProcessor_ApplyEnvoyPolicies_NonEmpty(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "envoy-allow")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.EnvoyPolicies, 1)

	req, err := LoadEnvoyRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	rc := &ResultCounts{}
	processor := NewAuthzProcessor(rc, nil, nil, res.EnvoyPolicies, false)
	responses, err := processor.ApplyEnvoyPolicies([]*authv3.CheckRequest{req})
	require.NoError(t, err)
	assert.Len(t, responses, 1)
	assert.Equal(t, 1, rc.Pass)
}

// TestAuthzProcessor_ApplyEnvoyPolicies_Deny verifies that ApplyEnvoyPolicies handles
// denied Envoy authorization policies and honors the auditWarn configuration.
func TestAuthzProcessor_ApplyEnvoyPolicies_Deny(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "envoy-deny")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.EnvoyPolicies, 1)

	req, err := LoadEnvoyRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	t.Run("auditWarn false with audit action counts failure as fail", func(t *testing.T) {
		p := *res.EnvoyPolicies[0]
		p.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}

		rc := &ResultCounts{}
		processor := NewAuthzProcessor(rc, nil, nil, []*policiesv1beta1.ValidatingPolicy{&p}, false)
		responses, err := processor.ApplyEnvoyPolicies([]*authv3.CheckRequest{req})
		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, 1, rc.Fail)
		assert.Equal(t, 0, rc.Warn)
		assert.Equal(t, 0, rc.Error)
	})

	t.Run("auditWarn true with audit action counts failure as warn", func(t *testing.T) {
		p := *res.EnvoyPolicies[0]
		p.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}

		rc := &ResultCounts{}
		processor := NewAuthzProcessor(rc, nil, nil, []*policiesv1beta1.ValidatingPolicy{&p}, true)
		responses, err := processor.ApplyEnvoyPolicies([]*authv3.CheckRequest{req})
		require.NoError(t, err)
		assert.Len(t, responses, 1)
		assert.Equal(t, 0, rc.Fail)
		assert.Equal(t, 1, rc.Warn)
		assert.Equal(t, 0, rc.Error)
	})
}

func testAuthzFixturePolicyDir(t *testing.T, subdir string) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "test", "cli", "test-validating-policy", subdir)
	_, err := os.Stat(dir)
	require.NoError(t, err, "fixture dir missing: %s", dir)
	return dir
}

func TestProcessHTTPPolicy_Allow(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "http-allow")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.HTTPPolicies, 1)

	req, err := LoadHTTPRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	resp, err := processHTTPPolicy(res.HTTPPolicies[0], req, nil)
	require.NoError(t, err)
	require.Len(t, resp.PolicyResponse.Rules, 1)
	assert.Equal(t, engineapi.RuleStatusPass, resp.PolicyResponse.Rules[0].Status())
}

func TestProcessHTTPPolicy_Deny(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "http-deny")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.HTTPPolicies, 1)

	req, err := LoadHTTPRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	resp, err := processHTTPPolicy(res.HTTPPolicies[0], req, nil)
	require.NoError(t, err)
	require.Len(t, resp.PolicyResponse.Rules, 1)
	assert.Equal(t, engineapi.RuleStatusFail, resp.PolicyResponse.Rules[0].Status())
}

func TestProcessEnvoyPolicy_Allow(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "envoy-allow")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.EnvoyPolicies, 1)

	req, err := LoadEnvoyRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	resp, err := processEnvoyPolicy(res.EnvoyPolicies[0], req, nil)
	require.NoError(t, err)
	require.Len(t, resp.PolicyResponse.Rules, 1)
	assert.Equal(t, engineapi.RuleStatusPass, resp.PolicyResponse.Rules[0].Status())
}

func TestProcessEnvoyPolicy_Deny(t *testing.T) {
	dir := testAuthzFixturePolicyDir(t, "envoy-deny")
	res, err := policy.Load(nil, "", filepath.Join(dir, "policy.yaml"))
	require.NoError(t, err)
	require.Len(t, res.EnvoyPolicies, 1)

	req, err := LoadEnvoyRequests(filepath.Join(dir, "request.json"))
	require.NoError(t, err)

	resp, err := processEnvoyPolicy(res.EnvoyPolicies[0], req, nil)
	require.NoError(t, err)
	require.Len(t, resp.PolicyResponse.Rules, 1)
	assert.Equal(t, engineapi.RuleStatusFail, resp.PolicyResponse.Rules[0].Status())
}

func TestProcessHTTPPolicy_CompileError(t *testing.T) {
	badYAML := `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: bad-http
spec:
  evaluation:
    mode: HTTP
  validations:
  - expression: "1 +"
`
	policyPath := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(badYAML), 0o600))

	res, err := policy.Load(nil, "", policyPath)
	require.NoError(t, err)
	require.Len(t, res.HTTPPolicies, 1)

	req := &authzhttp.CheckRequest{}
	_, err = processHTTPPolicy(res.HTTPPolicies[0], req, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to compile HTTP policy")
}

func TestProcessEnvoyPolicy_CompileError(t *testing.T) {
	badYAML := `apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: bad-envoy
spec:
  evaluation:
    mode: Envoy
  validations:
  - expression: "1 +"
`
	policyPath := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(badYAML), 0o600))

	res, err := policy.Load(nil, "", policyPath)
	require.NoError(t, err)
	require.Len(t, res.EnvoyPolicies, 1)

	req := &authv3.CheckRequest{}
	_, err = processEnvoyPolicy(res.EnvoyPolicies[0], req, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to compile envoy policy")
}
