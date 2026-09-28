package exception

import (
	"os"
	"path/filepath"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_load(t *testing.T) {
	tests := []struct {
		name                string
		policies            string
		exceptionsLoaded    int
		celExceptionsLoaded int
		wantErr             bool
	}{{
		name:     "not a policy exception",
		policies: "../_testdata/resources/namespace.yaml",
		wantErr:  true,
	}, {
		name:                "policy exception",
		policies:            "../_testdata/exceptions/exception.yaml",
		exceptionsLoaded:    1,
		celExceptionsLoaded: 0,
	}, {
		// The PolicyException document loads fine; the unsupported Policy document that follows
		// it becomes the pending fatal error, but doesn't discard what was already loaded before
		// it -- scanning still continues so a legacy kind later in the same file can still be
		// blocked ahead of an earlier, unrelated error.
		name:             "policy exception and policy",
		policies:         "../_testdata/exceptions/exception-and-policy.yaml",
		exceptionsLoaded: 1,
		wantErr:          true,
	}, {
		name:                "cel policy exception",
		policies:            "../_testdata/exceptions/celexception.yaml",
		exceptionsLoaded:    0,
		celExceptionsLoaded: 1,
	}, {
		name:                "cel policy exception and policy",
		policies:            "../_testdata/exceptions/exception-and-celexception.yaml",
		exceptionsLoaded:    1,
		celExceptionsLoaded: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := os.ReadFile(tt.policies)
			require.NoError(t, err)
			require.NoError(t, err)
			if res, err := load(bytes, true); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			} else if res != nil {
				if len(res.Exceptions) != tt.exceptionsLoaded {
					t.Errorf("Load() loaded amount = %v, wantLoaded %v", len(res.Exceptions), tt.exceptionsLoaded)
				} else if len(res.CELExceptions) != tt.celExceptionsLoaded {
					t.Errorf("Load() loaded amount = %v, wantLoaded %v", len(res.CELExceptions), tt.celExceptionsLoaded)
				}
			}
		})
	}
}

func Test_Load_BlocksAllLegacyExceptionVersions(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
	}{
		{
			name: "PolicyException v2",
			manifest: `
apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: test-exception
  namespace: default
spec:
  exceptions:
  - policyName: test-policy
    ruleNames:
    - test-rule
  match:
    any:
    - resources:
        kinds:
        - Pod
`,
		},
		{
			name: "PolicyException v2beta1",
			manifest: `
apiVersion: kyverno.io/v2beta1
kind: PolicyException
metadata:
  name: test-exception
  namespace: default
spec:
  exceptions:
  - policyName: test-policy
    ruleNames:
    - test-rule
  match:
    any:
    - resources:
        kinds:
        - Pod
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "exception.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tt.manifest), 0o600))

			_, err := Load(false, path)
			require.Error(t, err, "expected the legacy-policy block to fire")
			assert.Contains(t, err.Error(), "is no longer accepted")

			_, err = Load(true, path)
			require.NoError(t, err, "expected allowLegacyPolicies to bypass the block")
		})
	}
}

func Test_load_BlocksLegacyExceptionAfterUnsupportedDocument(t *testing.T) {
	// An unsupported document (a legacy Policy, which this loader doesn't handle at all) ahead of
	// a legacy PolicyException in the same multi-document file must not cause the loader to give
	// up before it reaches the legacy exception: the block still has to fire for it, taking
	// priority over the earlier, unrelated "policy exception type not supported" error.
	content := []byte(`
apiVersion: kyverno.io/v1
kind: Policy
metadata:
  name: unsupported-here
  namespace: test
spec:
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
---
apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: test-exception
  namespace: default
spec:
  exceptions:
  - policyName: test-policy
    ruleNames:
    - test-rule
  match:
    any:
    - resources:
        kinds:
        - Pod
`)

	_, err := load(content, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException is no longer accepted")

	_, err = load(content, true)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "no longer accepted")
	assert.Contains(t, err.Error(), "policy exception type not supported")
}

func Test_load_BlocksMalformedLegacyException(t *testing.T) {
	// GVK parses fine (kyverno.io/v2 PolicyException) but `exceptions` has the wrong type, so
	// OpenAPI schema validation fails. The loader still returns the parsed GVK alongside that
	// error, so the block must fire instead of surfacing only the generic validation error.
	malformed := []byte(`
apiVersion: kyverno.io/v2
kind: PolicyException
metadata:
  name: malformed
  namespace: default
spec:
  exceptions: "not-a-list"
`)

	_, err := load(malformed, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException is no longer accepted")

	_, err = load(malformed, true)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "no longer accepted")
}

func Test_SelectFrom(t *testing.T) {
	resources := toUnstructured(t,
		&corev1.ConfigMap{TypeMeta: v1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}},
		&kyvernov2.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: exceptionV2.Kind, APIVersion: exceptionV2.GroupVersion().String()},
		},
		&kyvernov2beta1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: exceptionV2beta1.Kind, APIVersion: exceptionV2beta1.GroupVersion().String()},
		},
		&policiesv1beta1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: celExceptionV1beta1.Kind, APIVersion: celExceptionV1beta1.GroupVersion().String()},
		},
		&policiesv1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: celExceptionV1.Kind, APIVersion: celExceptionV1.GroupVersion().String()},
		},
	)
	results, err := SelectFrom(resources, true)
	require.NoError(t, err)
	require.Len(t, results.Exceptions, 2)
	require.Len(t, results.CELExceptions, 2)
}

func Test_SelectFrom_MalformedExceptionSurfacesError(t *testing.T) {
	// A malformed exception (recognized GVK, but a field of the wrong type) must not be silently
	// dropped -- it should be surfaced as an error, without preventing a later, valid exception in
	// the same --resource batch from still being picked up.
	malformed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": exceptionV2.GroupVersion().String(),
		"kind":       exceptionV2.Kind,
		"metadata": map[string]interface{}{
			"name": "malformed",
		},
		"spec": map[string]interface{}{
			"exceptions": "not-a-list",
		},
	}}
	valid := toUnstructured(t, &kyvernov2beta1.PolicyException{TypeMeta: v1.TypeMeta{
		Kind: exceptionV2beta1.Kind, APIVersion: exceptionV2beta1.GroupVersion().String()},
	})
	resources := append([]*unstructured.Unstructured{malformed}, valid...)

	results, err := SelectFrom(resources, true)
	require.Error(t, err)
	require.Len(t, results.Exceptions, 1, "the valid exception after the malformed one should still be picked up")
}

func Test_SelectFrom_MalformedCELExceptionSurfacesError(t *testing.T) {
	// Same as Test_SelectFrom_MalformedExceptionSurfacesError, but for the CEL PolicyException
	// branch: a malformed CEL exception must not be silently dropped, and a later, valid CEL
	// exception in the same --resource batch must still be picked up.
	malformed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": celExceptionV1.GroupVersion().String(),
		"kind":       celExceptionV1.Kind,
		"metadata": map[string]interface{}{
			"name": "malformed",
		},
		"spec": map[string]interface{}{
			"exceptions": "not-a-list",
		},
	}}
	valid := toUnstructured(t, &policiesv1beta1.PolicyException{TypeMeta: v1.TypeMeta{
		Kind: celExceptionV1beta1.Kind, APIVersion: celExceptionV1beta1.GroupVersion().String()},
	})
	resources := append([]*unstructured.Unstructured{malformed}, valid...)

	results, err := SelectFrom(resources, true)
	require.Error(t, err)
	require.Len(t, results.CELExceptions, 1, "the valid CEL exception after the malformed one should still be picked up")
}

func Test_SelectFrom_BlocksLegacyException(t *testing.T) {
	resources := toUnstructured(t,
		&kyvernov2.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: exceptionV2.Kind, APIVersion: exceptionV2.GroupVersion().String()},
		},
	)

	_, err := SelectFrom(resources, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException is no longer accepted")

	results, err := SelectFrom(resources, true)
	require.NoError(t, err)
	require.Len(t, results.Exceptions, 1)
}

func toUnstructured(t *testing.T, in ...interface{}) []*unstructured.Unstructured {
	var resources []*unstructured.Unstructured
	for _, r := range in {
		us, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
		require.NoError(t, err)
		resources = append(resources, &unstructured.Unstructured{Object: us})
	}

	return resources
}
