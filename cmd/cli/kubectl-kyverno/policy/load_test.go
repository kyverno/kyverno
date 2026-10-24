package policy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
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

// TestLoad_CELExceptionUnknownField checks that a CEL PolicyException with an unknown field fails
// the strict load.
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

// TestLoadSkippingInvalidFiles checks that broken policies are reported by file and name while valid
// policies and non policy files around them are handled as expected.
func TestLoadSkippingInvalidFiles(t *testing.T) {
	const validPolicy = `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "has(object.metadata.labels) && 'team' in object.metadata.labels"
`
	const unknownField = `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: typo
spec:
  validationActionz: [Deny]
  validations:
  - expression: "true"
`
	const pod = `
apiVersion: v1
kind: Pod
metadata:
  name: example
spec:
  containers:
  - name: nginx
    image: nginx
`
	dir := t.TempDir()
	for name, content := range map[string]string{
		"a-typo.yaml":           unknownField,
		"b-valid.yaml":          validPolicy,
		"c-pod.yaml":            pod,
		"d-broken.yaml":         "apiVersion: policies.kyverno.io/v1\nkind: ValidatingPolicy\nmetadata:\n  name: [broken\n",
		"e-newer.yaml":          strings.Replace(validPolicy, "policies.kyverno.io/v1", "policies.kyverno.io/v9", 1),
		"f-mixed.yaml":          strings.Replace(validPolicy, "require-team-label", "require-team-label-2", 1) + "---\n" + pod,
		"g-valid-and-typo.yaml": strings.Replace(validPolicy, "require-team-label", "require-team-label-3", 1) + "---\n" + unknownField + "---\n" + strings.Replace(unknownField, "name: typo", "name: typo-2", 1),
		"kustomization.yaml":    "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- b-valid.yaml\n",
		"values.yaml":           "policies:\n- name: require-team-label\n",
		"notes.txt":             "not yaml",
		".hidden.yaml":          unknownField,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	// the strict loader still fails the whole directory on the first broken file
	_, err := Load(nil, "", dir)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", dir)
	require.NoError(t, err)
	// the valid policy file, the policy sharing a file with a Pod and the valid policy sharing a
	// file with a broken one are all loaded
	assert.Len(t, results.ValidatingPolicies, 3)
	// only policy documents that failed are reported, the Pod, kustomization and values file are not
	require.Len(t, results.NonFatalErrors, 5)
	assert.Equal(t, filepath.Join(dir, "a-typo.yaml"), results.NonFatalErrors[0].Path)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "unknown field")
	assert.Equal(t, filepath.Join(dir, "d-broken.yaml"), results.NonFatalErrors[1].Path)
	assert.ErrorContains(t, results.NonFatalErrors[1].Error, "yaml")
	assert.Equal(t, filepath.Join(dir, "e-newer.yaml"), results.NonFatalErrors[2].Path)
	assert.ErrorContains(t, results.NonFatalErrors[2].Error, "policies.kyverno.io/v9")
	// both broken policies in g-valid-and-typo.yaml are reported, each named, not only the first one
	for i, name := range map[int]string{3: "ValidatingPolicy typo: ", 4: "ValidatingPolicy typo-2: "} {
		assert.Equal(t, filepath.Join(dir, "g-valid-and-typo.yaml"), results.NonFatalErrors[i].Path)
		assert.ErrorContains(t, results.NonFatalErrors[i].Error, "unknown field")
		assert.True(t, strings.HasPrefix(results.NonFatalErrors[i].Error.Error(), name), results.NonFatalErrors[i].Error.Error())
	}
}

// TestIsPolicyGroupKind checks which kinds are treated as policies when deciding what to report.
func TestIsPolicyGroupKind(t *testing.T) {
	tests := []struct {
		gvk  schema.GroupVersionKind
		want bool
	}{
		{gvk: schema.GroupVersionKind{Group: "policies.kyverno.io", Version: "v1", Kind: "ValidatingPolicy"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "policies.kyverno.io", Version: "v9", Kind: "SomethingNew"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "kyverno.io", Version: "v1", Kind: "ClusterPolicy"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "CleanupPolicy"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "PolicyException"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "kyverno.io", Version: "v2beta1", Kind: "GlobalContextEntry"}, want: false},
		{gvk: schema.GroupVersionKind{Group: "kyverno.io", Version: "v2", Kind: "UpdateRequest"}, want: false},
		{gvk: schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingAdmissionPolicy"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingAdmissionPolicyBinding"}, want: true},
		{gvk: schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"}, want: false},
		{gvk: schema.GroupVersionKind{Group: "cli.kyverno.io", Version: "v1alpha1", Kind: "Test"}, want: false},
		{gvk: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, want: false},
		{gvk: schema.GroupVersionKind{}, want: false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, isPolicyGroupKind(tt.gvk), tt.gvk.String())
	}
}

// TestLoadSkippingInvalidFiles_UnreadableEntries checks that a dangling symlink is reported without
// dropping the rest of the directory, and that hidden and non YAML entries are ignored.
func TestLoadSkippingInvalidFiles_UnreadableEntries(t *testing.T) {
	dir := t.TempDir()
	valid := `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "true"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(valid), 0o600))
	missing := filepath.Join(dir, "does-not-exist")
	require.NoError(t, os.Symlink(missing, filepath.Join(dir, "dangling.yaml")))
	require.NoError(t, os.Symlink(missing, filepath.Join(dir, "README.md")))
	require.NoError(t, os.Symlink(missing, filepath.Join(dir, ".hidden.yaml")))

	// the strict loader still fails the whole directory
	_, err := Load(nil, "", dir)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", dir)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	// only the dangling YAML entry is reported, the non YAML and hidden ones are ignored
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, filepath.Join(dir, "dangling.yaml"), results.NonFatalErrors[0].Path)
}

// TestLoadSkippingInvalidFiles_SymlinkedDirectories checks that a symlink back to a directory being
// loaded is reported once instead of loading its policies again on every level, and that a dangling
// symlink that could have pointed at a directory is reported.
func TestLoadSkippingInvalidFiles_SymlinkedDirectories(t *testing.T) {
	dir := t.TempDir()
	valid := `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "true"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(valid), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
	require.NoError(t, os.Symlink("..", filepath.Join(dir, "nested", "parent")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "does-not-exist"), filepath.Join(dir, "policies")))

	results, err := LoadSkippingInvalidFiles(nil, "", dir)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1, "the policy must be loaded exactly once")
	require.Len(t, results.NonFatalErrors, 2)
	assert.Equal(t, filepath.Join(dir, "nested", "parent"), results.NonFatalErrors[0].Path)
	assert.Contains(t, results.NonFatalErrors[0].Error.Error(), "symlink loop")
	assert.Equal(t, filepath.Join(dir, "policies"), results.NonFatalErrors[1].Path)
}

// TestLoadSkippingInvalidFiles_SymlinkLoopFromRelativePath checks that a loop is caught on its first
// level when the directory is given as a relative path and the symlink back to it is absolute.
func TestLoadSkippingInvalidFiles_SymlinkLoopFromRelativePath(t *testing.T) {
	dir := t.TempDir()
	valid := `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: require-team-label
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [CREATE]
      resources: [pods]
  validations:
  - expression: "true"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(valid), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o700))
	require.NoError(t, os.Symlink(dir, filepath.Join(dir, "nested", "parent")))
	wd, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(wd, dir)
	require.NoError(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", relative)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1, "the policy must be loaded exactly once")
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, filepath.Join(relative, "nested", "parent"), results.NonFatalErrors[0].Path)
	assert.Contains(t, results.NonFatalErrors[0].Error.Error(), "symlink loop")
}

// TestLoadSkippingInvalidFiles_ValidatorSetupErrorIsFatal checks that a validator that cannot be set
// up fails the load instead of being reported against every file.
func TestLoadSkippingInvalidFiles_ValidatorSetupErrorIsFatal(t *testing.T) {
	original := loaderDelegate
	t.Cleanup(func() { loaderDelegate = original })
	loaderDelegate = func() (resourceloader.Loader, error) {
		return nil, errors.New("failed to build validator")
	}

	_, err := LoadSkippingInvalidFiles(nil, "", "testdata/list-vaps.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorSetup)
	assert.Contains(t, err.Error(), "failed to build validator")
}

// TestDocumentError checks how errors are prefixed with the kind and name of their document.
func TestDocumentError(t *testing.T) {
	base := errors.New("boom")
	named := documentError([]byte("apiVersion: policies.kyverno.io/v1\nkind: ValidatingPolicy\nmetadata:\n  name: check\n"), base)
	assert.EqualError(t, named, "ValidatingPolicy check: boom")
	assert.ErrorIs(t, named, base)
	assert.EqualError(t, namedError("ValidatingPolicy", "check", base), "ValidatingPolicy check: boom")
	assert.Equal(t, base, namedError("", "check", base))
	assert.Equal(t, base, namedError("ValidatingPolicy", "", base))
	// documents without a readable kind and name keep the original error
	assert.Equal(t, base, documentError([]byte("name: [broken"), base))
	assert.Equal(t, base, documentError([]byte("kind: ValidatingPolicy\n"), base))
}

// TestSkipInvalidFiles_KeepsPartialResults checks that the wrapper keeps what a loader returned
// alongside its error.
func TestSkipInvalidFiles_KeepsPartialResults(t *testing.T) {
	partial := &LoaderResults{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{&policiesv1beta1.ValidatingPolicy{}},
		Warnings:           []LoaderWarning{{Path: "mixed.yaml", Warning: "deprecated"}},
		NonFatalErrors:     []LoaderError{{Path: "mixed.yaml", Error: errors.New("document error")}},
	}
	l := skipInvalidFiles(func(string, []byte) (*LoaderResults, error) {
		return partial, errors.New("file error")
	})

	results, err := l("mixed.yaml", nil)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	assert.Len(t, results.Warnings, 1)
	require.Len(t, results.NonFatalErrors, 2)
	assert.EqualError(t, results.NonFatalErrors[1].Error, "file error")

	// a loader that returns no results at all still gets its error recorded
	l = skipInvalidFiles(func(string, []byte) (*LoaderResults, error) {
		return nil, errors.New("cannot split documents")
	})
	results, err = l("broken.yaml", nil)
	require.NoError(t, err)
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, "broken.yaml", results.NonFatalErrors[0].Path)
}

// schemaLessLoader accepts any document without schema validation, which lets tests reach the
// document processing step with kinds the real validator would reject earlier.
type schemaLessLoader struct{}

// Load returns the document as is with its declared GVK, without any schema validation.
func (schemaLessLoader) Load(document []byte) (schema.GroupVersionKind, unstructured.Unstructured, error) {
	var untyped unstructured.Unstructured
	if err := yaml.Unmarshal(document, &untyped.Object); err != nil {
		return schema.GroupVersionKind{}, untyped, err
	}
	return untyped.GroupVersionKind(), untyped, nil
}

// TestLoadSkippingInvalidFiles_ReportsPolicyDocumentsThatCannotBeProcessed checks that a policy
// document that passes validation but cannot be converted is reported, while a ConfigMap is not.
func TestLoadSkippingInvalidFiles_ReportsPolicyDocumentsThatCannotBeProcessed(t *testing.T) {
	original := loaderDelegate
	t.Cleanup(func() { loaderDelegate = original })
	loaderDelegate = func() (resourceloader.Loader, error) { return schemaLessLoader{}, nil }

	content := `apiVersion: policies.kyverno.io/v1
kind: SomethingNew
metadata:
  name: newer
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-policy
`
	path := filepath.Join(t.TempDir(), "mixed.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	// the strict loader fails the file on the first document it cannot process
	_, err := Load(nil, "", path)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", path)
	require.NoError(t, err)
	// the unknown policy kind is reported, the ConfigMap is not
	require.Len(t, results.NonFatalErrors, 1)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "SomethingNew")
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "policy type not supported")
}

// TestLoadSkippingInvalidFiles_NamesFailingListItems checks that a failing policy inside a List is
// reported by kind and name, while the valid item still loads and a Pod is ignored.
func TestLoadSkippingInvalidFiles_NamesFailingListItems(t *testing.T) {
	content := `apiVersion: v1
kind: List
items:
- apiVersion: policies.kyverno.io/v1
  kind: ValidatingPolicy
  metadata:
    name: valid
  spec:
    validations:
    - expression: "true"
- apiVersion: policies.kyverno.io/v1
  kind: SomethingNew
  metadata:
    name: newer
- apiVersion: v1
  kind: Pod
  metadata:
    name: example
`
	path := filepath.Join(t.TempDir(), "list.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	results, err := LoadSkippingInvalidFiles(nil, "", path)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	// the unknown policy kind is reported by kind and name, the Pod is ignored
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, path, results.NonFatalErrors[0].Path)
	assert.True(t, strings.HasPrefix(results.NonFatalErrors[0].Error.Error(), "SomethingNew newer: failed to process List item 1"), results.NonFatalErrors[0].Error.Error())
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "policy type not supported")
}

// TestLoadSkippingInvalidFiles_KeepsOtherPathsWhenOneFails checks that a missing path is reported
// without dropping the policies loaded from the other paths.
func TestLoadSkippingInvalidFiles_KeepsOtherPathsWhenOneFails(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.yaml")
	require.NoError(t, os.WriteFile(valid, []byte(`apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: valid
spec:
  validations:
  - expression: "true"
`), 0o600))
	missing := filepath.Join(dir, "missing.yaml")

	// the strict loader fails the whole call
	_, err := Load(nil, "", valid, missing)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", missing, valid)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, missing, results.NonFatalErrors[0].Path)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "no such file or directory")
}

// TestLoadSkippingInvalidFiles_KeepsDocumentsReadBeforeSplitError checks that a read error part way
// through a file keeps the documents read before it and reports the rest of the file.
func TestLoadSkippingInvalidFiles_KeepsDocumentsReadBeforeSplitError(t *testing.T) {
	original := splitDocuments
	t.Cleanup(func() { splitDocuments = original })
	valid := []byte(`apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: valid
spec:
  validations:
  - expression: "true"
`)
	splitDocuments = func([]byte) ([][]byte, error) {
		return [][]byte{valid}, errors.New("unable to read yaml")
	}
	path := filepath.Join(t.TempDir(), "partial.yaml")
	require.NoError(t, os.WriteFile(path, valid, 0o600))

	_, err := Load(nil, "", path)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", path)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, path, results.NonFatalErrors[0].Path)
	assert.EqualError(t, results.NonFatalErrors[0].Error, "unable to read yaml")
}

// TestLoadSkippingInvalidFiles_IgnoresMalformedNonPolicyLists checks that a List whose shape is broken
// is not reported as a policy that could not be loaded, while the strict loader still records it.
func TestLoadSkippingInvalidFiles_IgnoresMalformedNonPolicyLists(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"scalar-item.yaml":    "apiVersion: v1\nkind: List\nitems:\n- just a string\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: example\n",
		"items-not-list.yaml": "apiVersion: v1\nkind: List\nitems: not a list\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	strict, err := Load(nil, "", filepath.Join(dir, "scalar-item.yaml"), filepath.Join(dir, "items-not-list.yaml"))
	require.NoError(t, err)
	assert.Len(t, strict.NonFatalErrors, 3)

	results, err := LoadSkippingInvalidFiles(nil, "", dir)
	require.NoError(t, err)
	assert.Empty(t, results.NonFatalErrors)
}

// TestLoadSkippingInvalidFiles_ReportsInvalidYAML checks that a document that is not valid YAML is
// reported even when it looks like a List, unlike a valid List whose shape is broken.
func TestLoadSkippingInvalidFiles_ReportsInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken-list.yaml")
	require.NoError(t, os.WriteFile(path, []byte("apiVersion: v1\nkind: List\nitems: [unclosed\n"), 0o600))

	results, err := LoadSkippingInvalidFiles(nil, "", path)
	require.NoError(t, err)
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, path, results.NonFatalErrors[0].Path)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "yaml")
}

// TestLoad_StrictListItemErrorsKeepTheirText checks that the strict loader reports a failing List item with
// the same error text as before, without the kind and name prefix the lenient loader adds.
func TestLoad_StrictListItemErrorsKeepTheirText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: v1
kind: List
items:
- apiVersion: policies.kyverno.io/v1
  kind: SomethingNew
  metadata:
    name: newer
`), 0o600))

	strict, err := Load(nil, "", path)
	require.NoError(t, err)
	require.Len(t, strict.NonFatalErrors, 1)
	assert.True(t, strings.HasPrefix(strict.NonFatalErrors[0].Error.Error(), "failed to process List item 0 ("), strict.NonFatalErrors[0].Error.Error())

	lenient, err := LoadSkippingInvalidFiles(nil, "", path)
	require.NoError(t, err)
	require.Len(t, lenient.NonFatalErrors, 1)
	assert.True(t, strings.HasPrefix(lenient.NonFatalErrors[0].Error.Error(), "SomethingNew newer: failed to process List item 0 ("), lenient.NonFatalErrors[0].Error.Error())
}

// TestLoadSkippingInvalidFiles_StillBlocksLegacyPolicies checks that a legacy policy still fails the
// lenient load.
func TestLoadSkippingInvalidFiles_StillBlocksLegacyPolicies(t *testing.T) {
	dir := t.TempDir()
	content, err := os.ReadFile("testdata/list-single-clusterpolicy.yaml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "legacy.yaml"), content, 0o600))

	_, err = LoadSkippingInvalidFiles(nil, "", dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy: Kyverno v1.20 removed execution")
}
