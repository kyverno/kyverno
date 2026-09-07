package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply_KindListResource(t *testing.T) {
	tempDir := t.TempDir()

	policyPath := filepath.Join(tempDir, "policy.yaml")
	policyContent := `apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Audit
  background: true
  rules:
    - name: check-for-labels
      match:
        any:
          - resources:
              kinds:
                - Pod
      validate:
        message: The label is required.
        pattern:
          metadata:
            labels:
              testlabel: "?*"
`
	require.NoError(t, os.WriteFile(policyPath, []byte(policyContent), 0o644))

	resourcePath := filepath.Join(tempDir, "list.yaml")
	resourceContent := `apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Service
  metadata:
    name: list-service-test
  spec:
    ports:
    - protocol: TCP
      port: 80
- apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: list-deployment-test
    labels:
      app: list-deployment-test
  spec:
    replicas: 1
    selector:
      matchLabels:
        app: list-deployment-test
    template:
      metadata:
        labels:
          app: list-deployment-test
      spec:
        containers:
          - name: nginx
            image: nginx
`
	require.NoError(t, os.WriteFile(resourcePath, []byte(resourceContent), 0o644))

	c := &ApplyCommandConfig{
		PolicyPaths:   []string{policyPath},
		ResourcePaths: []string{resourcePath},
	}

	var out bytes.Buffer
	rc, resources, skipped, _, err := c.applyCommandHelper(&out)
	require.NoError(t, err)
	require.NotNil(t, rc)
	assert.Empty(t, skipped.invalid)
	require.Len(t, resources, 2)
}
