package pull

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
)

const testClusterPolicyYAML = `
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

func TestExtractAndSavePolicies(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte(testClusterPolicyYAML))

	err := extractAndSavePolicies(layer, dir)
	assert.NoError(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed after extraction")

	out, err := os.ReadFile(filepath.Join(dir, "require-labels.yaml"))
	assert.NoError(t, err)
	assert.Contains(t, string(out), "require-labels")
}

func TestExtractAndSavePoliciesClosesReaderOnUnmarshalError(t *testing.T) {
	dir := t.TempDir()
	layer := newTrackedLayer(t, []byte("not: [valid, policy"))

	err := extractAndSavePolicies(layer, dir)
	assert.Error(t, err)
	assert.True(t, layer.rc.closed, "layer reader should be closed even when extraction fails")
}
