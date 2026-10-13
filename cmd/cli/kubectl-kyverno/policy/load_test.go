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
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
		name:         "cpol-limit-configmap-for-sa",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/cpol-limit-configmap-for-sa.yaml"},
		wantErr:      false,
	}, {
		name:         "invalid-schema",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/invalid-schema.yaml"},
		wantErr:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.fs, tt.resourcePath, true, tt.paths...)
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
			results, err := Load(tt.fs, tt.resourcePath, true, tt.paths...)
			if tt.wantErr {
				assert.NotNil(t, err, "result mismatch")
			} else {
				assert.NotNil(t, results)
				if results != nil {
					assert.Equal(t, tt.count, len(results.Policies), "policy count mismatch")
				}
			}
		})
	}
}

func TestLoadWithKubectlValidate(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		paths        []string
		wantErr      bool
		checks       func(*testing.T, []kyvernov1.PolicyInterface, []admissionregistrationv1.ValidatingAdmissionPolicy)
	}{{
		name:         "cpol-limit-configmap-for-sa",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/cpol-limit-configmap-for-sa.yaml"},
		wantErr:      false,
	}, {
		name:         "invalid-schema",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/invalid-schema.yaml"},
		wantErr:      true,
	}, {
		name:         "proper defaulting",
		fs:           nil,
		resourcePath: "",
		paths:        []string{"../_testdata/policies/check-image.yaml"},
		wantErr:      false,
		checks: func(t *testing.T, policies []kyvernov1.PolicyInterface, vaps []admissionregistrationv1.ValidatingAdmissionPolicy) {
			assert.Len(t, policies, 1)
			policy := policies[0]
			assert.NotNil(t, policy)
			spec := policy.GetSpec()
			assert.NotNil(t, spec)
			assert.True(t, spec.ValidationFailureAction.Audit())
			assert.NotNil(t, spec.Background)
			assert.True(t, *spec.Background)
			assert.NotNil(t, spec.Admission)
			assert.True(t, *spec.Admission)
			rule := spec.Rules[0]
			assert.Len(t, rule.VerifyImages, 1)
			assert.True(t, rule.VerifyImages[0].Required)
			assert.True(t, rule.VerifyImages[0].MutateDigest)
			assert.True(t, rule.VerifyImages[0].VerifyDigest)
			assert.True(t, rule.VerifyImages[0].UseCache)
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := LoadWithLoader(nil, tt.fs, tt.resourcePath, true, tt.paths...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checks != nil {
				tt.checks(t, results.Policies, results.VAPs)
			}
		})
	}
}

func TestKubectlValidateLoader_ListHandling(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		expectedPolicies int
		expectedVAPs     int
		expectedErrors   int
		wantErr          bool
	}{
		{
			name:             "list-single-clusterpolicy",
			path:             "testdata/list-single-clusterpolicy.yaml",
			expectedPolicies: 1,
			expectedVAPs:     0,
			expectedErrors:   0,
			wantErr:          false,
		},
		{
			name:             "list-multiple-policies",
			path:             "testdata/list-multiple-policies.yaml",
			expectedPolicies: 2,
			expectedVAPs:     0,
			expectedErrors:   0,
			wantErr:          false,
		},
		{
			name:             "list-empty",
			path:             "testdata/list-empty.yaml",
			expectedPolicies: 0,
			expectedVAPs:     0,
			expectedErrors:   0,
			wantErr:          false,
		},
		{
			name:             "list-mixed-items",
			path:             "testdata/list-mixed-items.yaml",
			expectedPolicies: 1,
			expectedVAPs:     0,
			expectedErrors:   1,
			wantErr:          false,
		},
		{
			name:             "list-vaps",
			path:             "testdata/list-vaps.yaml",
			expectedPolicies: 0,
			expectedVAPs:     1,
			expectedErrors:   0,
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Load(nil, "", true, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if results != nil {
				assert.Equal(t, tt.expectedPolicies, len(results.Policies), "policy count mismatch")
				assert.Equal(t, tt.expectedVAPs, len(results.VAPs), "VAP count mismatch")
				assert.Equal(t, tt.expectedErrors, len(results.NonFatalErrors), "error count mismatch")
				for i, policy := range results.Policies {
					t.Logf("Policy %d: %s/%s", i, policy.GetKind(), policy.GetName())
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

			_, err := Load(nil, "", false, path)
			require.Error(t, err, "expected the legacy-policy block to fire")
			assert.Contains(t, err.Error(), "is no longer accepted")

			_, err = Load(nil, "", true, path)
			require.NoError(t, err, "expected allowLegacyPolicies=true to bypass the block")
		})
	}
}

func TestLoad_BlocksMalformedLegacyClusterPolicy(t *testing.T) {
	// invalid-schema.yaml is a legacy kyverno.io/v1 ClusterPolicy that also fails OpenAPI schema
	// validation (unknown field). The loader still returns the parsed GVK alongside that error,
	// so the block must fire instead of surfacing only the generic schema error.
	_, err := Load(nil, "", false, "../_testdata/policies/invalid-schema.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy is no longer accepted")

	_, err = Load(nil, "", true, "../_testdata/policies/invalid-schema.yaml")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "no longer accepted")
}

func TestLoad_BlocksLegacyPolicyAfterUnsupportedDocumentInSameFile(t *testing.T) {
	// A plain ConfigMap ahead of a legacy ClusterPolicy in the same multi-document file must not
	// cause the loader to give up on the rest of the file before it reaches the legacy document:
	// the block still has to fire for the ClusterPolicy, taking priority over the ConfigMap's
	// (unrelated, non-fatal-by-comparison) "unsupported kind" error.
	_, err := Load(nil, "", false, "testdata/configmap-then-legacy-clusterpolicy.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy is no longer accepted")

	// With the block bypassed, the ConfigMap's own "unsupported kind" problem is still a real,
	// pre-existing error the loader must surface (not silently swallow), same as before this PR.
	_, err = Load(nil, "", true, "testdata/configmap-then-legacy-clusterpolicy.yaml")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "no longer accepted")
}

func TestLoad_BlocksLegacyPolicyInsideList(t *testing.T) {
	// testdata/list-single-clusterpolicy.yaml wraps a legacy kyverno.io/v1 ClusterPolicy in a
	// v1 List; the block must fire for a List item exactly as it does for a standalone manifest.
	_, err := Load(nil, "", false, "testdata/list-single-clusterpolicy.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy is no longer accepted")

	results, err := Load(nil, "", true, "testdata/list-single-clusterpolicy.yaml")
	require.NoError(t, err)
	assert.Len(t, results.Policies, 1)
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
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: test-policy
spec:
  rules: []
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

			results, err := Load(nil, "", true, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && len(results.Policies) != 1 {
				t.Errorf("expected 1 policy, got %d", len(results.Policies))
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

	_, err := Load(nil, "", true, server.URL)

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

			res, err := Load(nil, "", false, path)
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

	res, err := Load(nil, "", false, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid value: value provided for unknown field")
	if res != nil {
		assert.Empty(t, res.PolicyCelExceptions)
	}
}

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
	_, err := Load(nil, "", false, dir)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", false, dir)
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

func TestLoadSkippingInvalidFiles_KeepsWarningsFromPartiallyLoadedFile(t *testing.T) {
	legacy, err := os.ReadFile("testdata/list-single-clusterpolicy.yaml")
	require.NoError(t, err)
	broken := `
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: typo
spec:
  validationActionz: [Deny]
  validations:
  - expression: "true"
`
	path := filepath.Join(t.TempDir(), "mixed.yaml")
	require.NoError(t, os.WriteFile(path, append(legacy, []byte("---\n"+broken)...), 0o600))

	results, err := LoadSkippingInvalidFiles(nil, "", true, path)
	require.NoError(t, err)
	assert.Len(t, results.Policies, 1)
	assert.NotEmpty(t, results.Warnings)
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, path, results.NonFatalErrors[0].Path)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "unknown field")
}

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
	_, err := Load(nil, "", false, dir)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", false, dir)
	require.NoError(t, err)
	assert.Len(t, results.ValidatingPolicies, 1)
	// only the dangling YAML entry is reported, the non YAML and hidden ones are ignored
	require.Len(t, results.NonFatalErrors, 1)
	assert.Equal(t, filepath.Join(dir, "dangling.yaml"), results.NonFatalErrors[0].Path)
}

func TestLoadSkippingInvalidFiles_ValidatorSetupErrorIsFatal(t *testing.T) {
	original := loaderDelegate
	t.Cleanup(func() { loaderDelegate = original })
	loaderDelegate = func() (resourceloader.Loader, error) {
		return nil, errors.New("failed to build validator")
	}

	_, err := LoadSkippingInvalidFiles(nil, "", false, "testdata/list-vaps.yaml")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorSetup)
	assert.Contains(t, err.Error(), "failed to build validator")
}

func TestDocumentError(t *testing.T) {
	base := errors.New("boom")
	named := documentError([]byte("apiVersion: policies.kyverno.io/v1\nkind: ValidatingPolicy\nmetadata:\n  name: check\n"), base)
	assert.EqualError(t, named, "ValidatingPolicy check: boom")
	assert.ErrorIs(t, named, base)
	// documents without a readable kind and name keep the original error
	assert.Equal(t, base, documentError([]byte("name: [broken"), base))
	assert.Equal(t, base, documentError([]byte("kind: ValidatingPolicy\n"), base))
}

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

func (schemaLessLoader) Load(document []byte) (schema.GroupVersionKind, unstructured.Unstructured, error) {
	var untyped unstructured.Unstructured
	if err := yaml.Unmarshal(document, &untyped.Object); err != nil {
		return schema.GroupVersionKind{}, untyped, err
	}
	return untyped.GroupVersionKind(), untyped, nil
}

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
	_, err := Load(nil, "", false, path)
	require.Error(t, err)

	results, err := LoadSkippingInvalidFiles(nil, "", false, path)
	require.NoError(t, err)
	// the unknown policy kind is reported, the ConfigMap is not
	require.Len(t, results.NonFatalErrors, 1)
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "SomethingNew")
	assert.ErrorContains(t, results.NonFatalErrors[0].Error, "policy type not supported")
}

func TestLoadSkippingInvalidFiles_StillBlocksLegacyPolicies(t *testing.T) {
	dir := t.TempDir()
	content, err := os.ReadFile("testdata/list-single-clusterpolicy.yaml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "legacy.yaml"), content, 0o600))

	_, err = LoadSkippingInvalidFiles(nil, "", false, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v1 ClusterPolicy is no longer accepted")
}
