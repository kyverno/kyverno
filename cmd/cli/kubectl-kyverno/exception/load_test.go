package exception

import (
	"os"
	"path/filepath"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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
		celExceptionsLoaded int
		wantErr             bool
	}{{
		name:     "not a policy exception",
		policies: "../_testdata/resources/namespace.yaml",
		wantErr:  true,
	}, {
		// Legacy kyverno.io PolicyExceptions are unconditionally blocked.
		name:     "legacy policy exception",
		policies: "../_testdata/exceptions/exception.yaml",
		wantErr:  true,
	}, {
		name:     "legacy policy exception and policy",
		policies: "../_testdata/exceptions/exception-and-policy.yaml",
		wantErr:  true,
	}, {
		name:                "cel policy exception",
		policies:            "../_testdata/exceptions/celexception.yaml",
		celExceptionsLoaded: 1,
	}, {
		// The CEL exception document loads fine, but the legacy exception document that follows
		// it in the same file is unconditionally blocked -- the block aborts the whole load, so
		// nothing (not even the already-loaded CEL exception) is returned.
		name:     "cel policy exception and legacy policy exception",
		policies: "../_testdata/exceptions/exception-and-celexception.yaml",
		wantErr:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := os.ReadFile(tt.policies)
			require.NoError(t, err)
			if res, err := load(bytes); (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			} else if res != nil {
				if len(res.CELExceptions) != tt.celExceptionsLoaded {
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

			_, err := Load(path)
			require.Error(t, err, "expected the legacy-policy block to fire")
			assert.Contains(t, err.Error(), "Kyverno v1.20 removed execution")
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

	_, err := load(content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException: Kyverno v1.20 removed execution")
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

	_, err := load(malformed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException: Kyverno v1.20 removed execution")
}

func Test_SelectFrom(t *testing.T) {
	resources := toUnstructured(t,
		&corev1.ConfigMap{TypeMeta: v1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"}},
		&policiesv1alpha1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: CELExceptionV1alpha1.Kind, APIVersion: CELExceptionV1alpha1.GroupVersion().String()},
		},
		&policiesv1beta1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: CELExceptionV1beta1.Kind, APIVersion: CELExceptionV1beta1.GroupVersion().String()},
		},
		&policiesv1.PolicyException{TypeMeta: v1.TypeMeta{
			Kind: CELExceptionV1.Kind, APIVersion: CELExceptionV1.GroupVersion().String()},
		},
	)
	results, err := SelectFrom(resources)
	require.NoError(t, err)
	require.Len(t, results.CELExceptions, 3)
}

func Test_SelectFrom_MalformedLegacyExceptionStillBlocked(t *testing.T) {
	// A legacy exception is unconditionally blocked, even when it's also malformed (a field of the
	// wrong type) -- the block error takes priority over a generic conversion error, and a valid
	// CEL exception elsewhere in the same --resource batch doesn't change that the whole call fails.
	malformed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": ExceptionV2.GroupVersion().String(),
		"kind":       ExceptionV2.Kind,
		"metadata": map[string]interface{}{
			"name": "malformed",
		},
		"spec": map[string]interface{}{
			"exceptions": "not-a-list",
		},
	}}
	valid := toUnstructured(t, &policiesv1beta1.PolicyException{TypeMeta: v1.TypeMeta{
		Kind: CELExceptionV1beta1.Kind, APIVersion: CELExceptionV1beta1.GroupVersion().String()},
	})
	resources := append([]*unstructured.Unstructured{malformed}, valid...)

	_, err := SelectFrom(resources)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException: Kyverno v1.20 removed execution")
}

func Test_SelectFrom_MalformedCELExceptionSurfacesError(t *testing.T) {
	// A malformed CEL exception (recognized GVK, but a field of the wrong type) must not be
	// silently dropped -- it should be surfaced as an error, without preventing a later, valid
	// exception in the same --resource batch from still being picked up.
	malformed := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": CELExceptionV1.GroupVersion().String(),
		"kind":       CELExceptionV1.Kind,
		"metadata": map[string]interface{}{
			"name": "malformed",
		},
		"spec": map[string]interface{}{
			"exceptions": "not-a-list",
		},
	}}
	valid := toUnstructured(t, &policiesv1beta1.PolicyException{TypeMeta: v1.TypeMeta{
		Kind: CELExceptionV1beta1.Kind, APIVersion: CELExceptionV1beta1.GroupVersion().String()},
	})
	resources := append([]*unstructured.Unstructured{malformed}, valid...)

	results, err := SelectFrom(resources)
	require.Error(t, err)
	require.Len(t, results.CELExceptions, 1, "the valid CEL exception after the malformed one should still be picked up")
}

func Test_SelectFrom_BlocksLegacyException(t *testing.T) {
	resources := []*unstructured.Unstructured{{Object: map[string]interface{}{
		"apiVersion": ExceptionV2.GroupVersion().String(),
		"kind":       ExceptionV2.Kind,
		"metadata": map[string]interface{}{
			"name": "legacy-exception",
		},
	}}}

	_, err := SelectFrom(resources)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kyverno.io/v2 PolicyException: Kyverno v1.20 removed execution")
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
