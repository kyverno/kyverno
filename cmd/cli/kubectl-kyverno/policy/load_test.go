package policy

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		paths        []string
		wantErr      bool
	}{{
		name:         "vpol-valid",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/vpol-valid.yaml"},
		wantErr:      false,
	}, {
		// Legacy kyverno.io policy kinds are unconditionally blocked.
		name:         "legacy-cpol",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/cpol-limit-configmap-for-sa.yaml"},
		wantErr:      true,
	}, {
		name:         "invalid-schema",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/invalid-schema.yaml"},
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.fs, tt.resourcePath, tt.paths...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		paths        []string
		wantErr      bool
		count        int
	}{{
		name:         "invalid policy resources",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies-invalid/"},
		wantErr:      false,
		count:        0,
	}, {
		name:         "mixed policy resources",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies-mixed/"},
		wantErr:      false,
		count:        2,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Load(tt.fs, tt.resourcePath, tt.paths...)
			if tt.wantErr {
				assert.NotNil(t, err, "result mismatch")
			} else {
				assert.NotNil(t, results)
				if results != nil {
					assert.Equal(t, tt.count, len(results.ValidatingPolicies), "policy count mismatch")
				}
			}
		})
	}
}

func TestKubectlValidateLoader_ListHandling(t *testing.T) {
	tests := []struct {
		name                       string
		path                       string
		expectedValidatingPolicies int
		expectedVAPs               int
		expectedErrors             int
		wantErr                    bool
	}{
		{
			name:                       "list-single-validatingpolicy",
			path:                       "testdata/list-single-validatingpolicy.yaml",
			expectedValidatingPolicies: 1,
			expectedVAPs:               0,
			expectedErrors:             0,
			wantErr:                    false,
		},
		{
			name:                       "list-multiple-policies",
			path:                       "testdata/list-multiple-policies.yaml",
			expectedValidatingPolicies: 2,
			expectedVAPs:               0,
			expectedErrors:             0,
			wantErr:                    false,
		},
		{
			name:                       "list-empty",
			path:                       "testdata/list-empty.yaml",
			expectedValidatingPolicies: 0,
			expectedVAPs:               0,
			expectedErrors:             0,
			wantErr:                    false,
		},
		{
			name:                       "list-mixed-items",
			path:                       "testdata/list-mixed-items.yaml",
			expectedValidatingPolicies: 1,
			expectedVAPs:               0,
			expectedErrors:             1,
			wantErr:                    false,
		},
		{
			name:                       "list-vaps",
			path:                       "testdata/list-vaps.yaml",
			expectedValidatingPolicies: 0,
			expectedVAPs:               1,
			expectedErrors:             0,
			wantErr:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Load(nil, "", tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if results != nil {
				assert.Equal(t, tt.expectedValidatingPolicies, len(results.ValidatingPolicies), "policy count mismatch")
				assert.Equal(t, tt.expectedVAPs, len(results.VAPs), "VAP count mismatch")
				assert.Equal(t, tt.expectedErrors, len(results.NonFatalErrors), "error count mismatch")
				for i, policy := range results.ValidatingPolicies {
					t.Logf("Policy %d: %s", i, policy.GetName())
				}
				for i, vap := range results.VAPs {
					t.Logf("VAP %d: %s", i, vap.Name)
				}
			}
		})
	}
}

func TestLoad_BlocksAllLegacyKindsAndVersions(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{
			name: "Policy v1",
			manifest: `
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: test-policy
  namespace: default
spec:
  background: false
  rules:
  - name: test-rule
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "test"
      pattern:
        metadata:
          labels:
            app: "?*"
`,
		},
		{
			name: "Policy v2beta1",
			manifest: `
apiVersion: kyverno.io/v2beta1
kind: Policy
metadata:
  name: test-policy
  namespace: default
spec:
  background: false
  rules:
  - name: test-rule
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "test"
      pattern:
        metadata:
          labels:
            app: "?*"
`,
		},
		{
			name: "ClusterPolicy v2beta1",
			manifest: `
apiVersion: kyverno.io/v2beta1
kind: ClusterPolicy
metadata:
  name: test-cluster-policy
spec:
  background: false
  rules:
  - name: test-rule
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "test"
      pattern:
        metadata:
          labels:
            app: "?*"
`,
		},
		{
			name: "CleanupPolicy v2",
			manifest: `
apiVersion: kyverno.io/v2
kind: CleanupPolicy
metadata:
  name: test-cleanup
  namespace: default
spec:
  schedule: '*/1 * * * *'
  match:
    any:
    - resources:
        kinds:
        - Pod
`,
		},
		{
			name: "CleanupPolicy v2beta1",
			manifest: `
apiVersion: kyverno.io/v2beta1
kind: CleanupPolicy
metadata:
  name: test-cleanup
  namespace: default
spec:
  schedule: '*/1 * * * *'
  match:
    any:
    - resources:
        kinds:
        - Pod
`,
		},
		{
			name: "ClusterCleanupPolicy v2",
			manifest: `
apiVersion: kyverno.io/v2
kind: ClusterCleanupPolicy
metadata:
  name: test-cluster-cleanup
spec:
  schedule: '*/1 * * * *'
  match:
    any:
    - resources:
        kinds:
        - Namespace
`,
		},
		{
			name: "ClusterCleanupPolicy v2beta1",
			manifest: `
apiVersion: kyverno.io/v2beta1
kind: ClusterCleanupPolicy
metadata:
  name: test-cluster-cleanup
spec:
  schedule: '*/1 * * * *'
  match:
    any:
    - resources:
        kinds:
        - Namespace
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.manifest), 0o600))

			_, err := Load(nil, "", path)
			require.Error(t, err, "expected the legacy-policy block to fire")
			assert.Contains(t, err.Error(), "Kyverno v1.20 removed execution")
		})
	}
}

func TestLoad_BlocksMalformedLegacyClusterPolicy(t *testing.T) {
	// invalid-schema.yaml is a legacy kyverno.io/v1 ClusterPolicy that also fails OpenAPI schema
	// validation (unknown field). The loader still returns the parsed GVK alongside that error,
	// so the block must fire instead of surfacing only the generic schema error.
	_, err := Load(nil, "", "../_testdata/policies/invalid-schema.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy: Kyverno v1.20 removed execution")
}

func TestLoad_BlocksLegacyPolicyAfterUnsupportedDocumentInSameFile(t *testing.T) {
	// A plain ConfigMap ahead of a legacy ClusterPolicy in the same multi-document file must not
	// cause the loader to give up on the rest of the file before it reaches the legacy document:
	// the block still has to fire for the ClusterPolicy, taking priority over the ConfigMap's
	// (unrelated, non-fatal-by-comparison) "unsupported kind" error.
	_, err := Load(nil, "", "testdata/configmap-then-legacy-clusterpolicy.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy: Kyverno v1.20 removed execution")
}

func TestLoad_BlocksLegacyPolicyInsideList(t *testing.T) {
	// testdata/list-single-clusterpolicy.yaml wraps a legacy kyverno.io/v1 ClusterPolicy in a
	// v1 List; the block must fire for a List item exactly as it does for a standalone manifest.
	_, err := Load(nil, "", "testdata/list-single-clusterpolicy.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy: Kyverno v1.20 removed execution")
}

func TestLoadHTTP(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "successful fetch",
			statusCode: http.StatusOK,
			body: `
apiVersion: policies.kyverno.io/v1alpha1
kind: ValidatingPolicy
metadata:
  name: test-policy
spec:
  validations:
  - expression: "true"
    message: "always passes"
  matchConstraints:
    resourceRules:
    - operations: ["CREATE", "UPDATE"]
      apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
`,
			wantErr: false,
		},
		{
			name:       "non 200 response",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			results, err := Load(nil, "", server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && len(results.ValidatingPolicies) != 1 {
				t.Errorf("expected 1 policy, got %d", len(results.ValidatingPolicies))
			}
		})
	}
}

func TestLoadHTTPTimeout(t *testing.T) {
	oldTimeout := remoteHTTPTimeout
	defer func() {
		remoteHTTPTimeout = oldTimeout
	}()

	remoteHTTPTimeout = 100 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer server.Close()

	_, err := Load(nil, "", server.URL)

	assert.Error(t, err)
	check := err
	for {
		if te, ok := check.(interface{ Timeout() bool }); ok && te.Timeout() {
			break
		}
		if ue, ok := check.(interface{ Unwrap() error }); ok && ue.Unwrap() != nil {
			check = ue.Unwrap()
			continue
		}
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestLoad_CELExceptionVersions(t *testing.T) {
	manifests := []struct {
		version string
		content string
	}{
		{
			version: "v1alpha1",
			content: `
apiVersion: policies.kyverno.io/v1alpha1
kind: PolicyException
metadata:
  name: test-exception-v1alpha1
spec:
  policyRefs:
  - name: check-deployment-labels
    kind: ValidatingPolicy
  matchConditions:
  - name: check-namespace
    expression: "object.metadata.namespace == 'test-ns'"
`,
		},
		{
			version: "v1beta1",
			content: `
apiVersion: policies.kyverno.io/v1beta1
kind: PolicyException
metadata:
  name: test-exception-v1beta1
spec:
  policyRefs:
  - name: check-deployment-labels
    kind: ValidatingPolicy
  matchConditions:
  - name: check-namespace
    expression: "object.metadata.namespace == 'test-ns'"
`,
		},
		{
			version: "v1",
			content: `
apiVersion: policies.kyverno.io/v1
kind: PolicyException
metadata:
  name: test-exception-v1
spec:
  policyRefs:
  - name: check-deployment-labels
    kind: ValidatingPolicy
  matchConditions:
  - name: check-namespace
    expression: "object.metadata.namespace == 'test-ns'"
`,
		},
	}

	for _, m := range manifests {
		t.Run(m.version, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "exception.yaml")
			require.NoError(t, os.WriteFile(path, []byte(m.content), 0o600))

			res, err := Load(nil, "", path)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Len(t, res.PolicyCelExceptions, 1)
			assert.Equal(t, "test-exception-"+m.version, res.PolicyCelExceptions[0].GetName())
		})
	}
}

func TestLoad_CELExceptionUnknownField(t *testing.T) {
	manifest := `
apiVersion: policies.kyverno.io/v1
kind: PolicyException
metadata:
  name: test-exception-unknown-field
spec:
  unknownField: invalid
  policyRefs:
  - name: check-deployment-labels
    kind: ValidatingPolicy
  matchConditions:
  - name: check-namespace
    expression: "object.metadata.namespace == 'test-ns'"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "exception.yaml")
	require.NoError(t, os.WriteFile(path, []byte(manifest), 0o600))

	res, err := Load(nil, "", path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid value: value provided for unknown field")
	if res != nil {
		assert.Empty(t, res.PolicyCelExceptions)
	}
}
