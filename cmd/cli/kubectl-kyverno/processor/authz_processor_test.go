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
	processor := NewAuthzProcessor(&ResultCounts{}, nil, []*policiesv1beta1.ValidatingPolicy{}, nil)

	responses, err := processor.ApplyHTTPPolicies(nil)
	require.NoError(t, err)
	assert.Len(t, responses, 0)

	responses, err = processor.ApplyHTTPPolicies([]*authzhttp.CheckRequest{})
	require.NoError(t, err)
	assert.Len(t, responses, 0)
}

func TestAuthzProcessor_ApplyEnvoyPolicies_EmptyInputs(t *testing.T) {
	processor := NewAuthzProcessor(&ResultCounts{}, nil, nil, []*policiesv1beta1.ValidatingPolicy{})

	responses, err := processor.ApplyEnvoyPolicies(nil)
	require.NoError(t, err)
	assert.Len(t, responses, 0)

	responses, err = processor.ApplyEnvoyPolicies([]*authv3.CheckRequest{})
	require.NoError(t, err)
	assert.Len(t, responses, 0)
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
