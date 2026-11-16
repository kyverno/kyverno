package framework

import (
	"os"
	"path/filepath"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

const validPolicyYAML = `apiVersion: policies.kyverno.io/v1beta1
kind: GeneratingPolicy
metadata:
  name: generate-secret
spec:
  evaluation:
    synchronize:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE", "UPDATE"]
      resources: ["namespaces"]
`

const validSecretYAML = `apiVersion: v1
kind: Secret
metadata:
  name: source-secret
  namespace: default
data:
  foo: YmFy
`

// writeFixture writes content to a temp file and returns its path.
func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadGeneratingPolicy(t *testing.T) {
	policy := LoadGeneratingPolicy(t, writeFixture(t, validPolicyYAML))
	assert.Equal(t, "generate-secret", policy.GetName())
	require.NotNil(t, policy.Spec.EvaluationConfiguration)
	require.NotNil(t, policy.Spec.EvaluationConfiguration.SynchronizationConfiguration)
	require.NotNil(t, policy.Spec.EvaluationConfiguration.SynchronizationConfiguration.Enabled)
	// confirms the *bool synchronize.enabled pointer field round-trips
	assert.True(t, *policy.Spec.EvaluationConfiguration.SynchronizationConfiguration.Enabled)
}

func TestLoadResource(t *testing.T) {
	secret := &corev1.Secret{}
	LoadResource(t, writeFixture(t, validSecretYAML), secret)
	assert.Equal(t, "source-secret", secret.GetName())
	assert.Equal(t, "default", secret.GetNamespace())
	// confirms base64 data decodes back to raw bytes
	assert.Equal(t, "bar", string(secret.Data["foo"]))
}

func TestDecodeSingleResource_Errors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		write   bool // false = pass a path that does not exist
		wantErr string
	}{
		{name: "missing file", write: false, wantErr: "load"},
		{name: "empty file", content: "", write: true, wantErr: "empty"},
		{name: "whitespace only", content: "\n   \n", write: true, wantErr: "empty"},
		{
			name:    "multi document",
			content: validSecretYAML + "---\n" + validSecretYAML,
			write:   true,
			wantErr: "documents",
		},
		{
			name:    "no name",
			content: "apiVersion: v1\nkind: Secret\nmetadata: {}\n",
			write:   true,
			wantErr: "no metadata.name",
		},
		{
			name:    "malformed yaml",
			content: "apiVersion: v1\nkind: Secret\n  bad: : indent\n",
			write:   true,
			wantErr: "load",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing.yaml")
			if tc.write {
				path = writeFixture(t, tc.content)
			}
			err := decodeSingleResource(path, &corev1.Secret{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestDecodeTypedResource_KindMismatch(t *testing.T) {
	// A Secret fixture decoded into a GeneratingPolicy decodes "successfully"
	// (shared ObjectMeta, lenient codec) but is semantically empty. The kind
	// guard must reject it rather than return a silent no-op policy.
	policy := &policiesv1beta1.GeneratingPolicy{}
	err := decodeTypedResource(writeFixture(t, validSecretYAML), policy, "GeneratingPolicy")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected kind GeneratingPolicy")
	assert.Contains(t, err.Error(), `got "Secret"`)

	// The matching kind passes.
	require.NoError(t, decodeTypedResource(writeFixture(t, validPolicyYAML), &policiesv1beta1.GeneratingPolicy{}, "GeneratingPolicy"))
}

func TestDecodeSingleResource_IgnoresUnknownFields(t *testing.T) {
	// Forward compatibility: a fixture carrying a field the current struct does
	// not know about should still decode (lenient), not error.
	content := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: s\nsomeFutureField: value\n"
	err := decodeSingleResource(writeFixture(t, content), &corev1.Secret{})
	assert.NoError(t, err)
}
