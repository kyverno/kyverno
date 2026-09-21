package pull

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
)

const testValidatingPolicyYAML = `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: require-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE", "UPDATE"]
  validations:
  - expression: "object.metadata.labels != null"
    message: "labels are required"
`

const testDeletingPolicyYAML = `
apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: delete-stale-pods
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
  conditions:
  - expression: "object.status.phase == 'Succeeded'"
`

const testLegacyClusterPolicyYAML = `
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  rules:
  - name: check-team
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
`

// trackedReadCloser records whether Close was called, so tests can assert
// that a layer's reader is actually released rather than only checking
// the returned error.
type trackedReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackedReadCloser) Close() error {
	t.closed = true
	return nil
}

// trackedLayer wraps a static layer, returning a trackedReadCloser from
// Compressed so tests can observe whether it was closed.
type trackedLayer struct {
	v1.Layer
	rc *trackedReadCloser
}

func newTrackedLayer(t *testing.T, data []byte) *trackedLayer {
	t.Helper()
	base := static.NewLayer(data, types.MediaType("test"))
	blob, err := base.Compressed()
	assert.NoError(t, err)
	return &trackedLayer{Layer: base, rc: &trackedReadCloser{Reader: blob}}
}

func (l *trackedLayer) Compressed() (io.ReadCloser, error) {
	return l.rc, nil
}

func TestExtractAndSavePoliciesValidatingPolicy(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte(testValidatingPolicyYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.NoError(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed after extraction")

	// Filename is now kind-prefixed to prevent collisions: validatingpolicy-<name>.yaml
	out, err := os.ReadFile(filepath.Join(dir, "validatingpolicy-require-labels.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(out), "require-labels")
	assert.Contains(t, string(out), "ValidatingPolicy")
}

func TestExtractAndSavePoliciesDeletingPolicy(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte(testDeletingPolicyYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.NoError(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed after extraction")

	// Filename is now kind-prefixed: deletingpolicy-<name>.yaml
	out, err := os.ReadFile(filepath.Join(dir, "deletingpolicy-delete-stale-pods.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(out), "delete-stale-pods")
}

func TestExtractAndSavePoliciesRejectsLegacyKinds(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte(testLegacyClusterPolicyYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "legacy"), "error should mention 'legacy'")
	assert.True(t, layer.rc.closed, "layer reader should be closed even when extraction fails")
}

func TestExtractAndSavePoliciesClosesReaderOnUnmarshalError(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte("not: [valid, policy"))

	err := extractAndSavePolicies(layer, dir)
	assert.Error(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed even when extraction fails")
}

func TestExtractAndSavePoliciesEmptyDocument(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte("   \n---\n   "))

	err := extractAndSavePolicies(layer, dir)
	assert.NoError(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed")
}

func TestExtractAndSavePoliciesRejectsUnknownResources(t *testing.T) {
	dir := t.TempDir()
	unknownYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
`
	layer := newTrackedLayer(t, []byte(unknownYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource")
	assert.True(t, layer.rc.closed, "layer reader should be closed even when extraction fails")
}

func TestExtractAndSavePoliciesRejectsVAP(t *testing.T) {
	dir := t.TempDir()
	// ValidatingAdmissionPolicy is a native k8s type in admissionregistration.k8s.io
	// and must be rejected even though it is a CEL-based type.
	vapYAML := `
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: check-labels
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      resources: ["pods"]
      operations: ["CREATE"]
  validations:
  - expression: "object.metadata.labels != null"
`
	layer := newTrackedLayer(t, []byte(vapYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource")
	assert.True(t, layer.rc.closed, "layer reader should be closed even when extraction fails")
}

func TestExtractAndSavePoliciesKindPrefixedFilenames(t *testing.T) {
	// Two policies of different kinds with the same object name must produce
	// two distinct files — no silent overwrite.
	dir := t.TempDir()
	multiYAML := `
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: same-name
spec:
  validations:
  - expression: "true"
---
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: same-name
`
	layer := newTrackedLayer(t, []byte(multiYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.NoError(t, err)

	_, errVP := os.Stat(filepath.Join(dir, "validatingpolicy-same-name.yaml"))
	assert.NoError(t, errVP, "expected validatingpolicy-same-name.yaml")
	_, errMP := os.Stat(filepath.Join(dir, "mutatingpolicy-same-name.yaml"))
	assert.NoError(t, errMP, "expected mutatingpolicy-same-name.yaml")
}
