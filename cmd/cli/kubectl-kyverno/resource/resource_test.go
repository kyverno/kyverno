package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYamlToUnstructured_HelmTemplates(t *testing.T) {
	rawYAML := `apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  labels:
    app: {{ .Chart.Name }}
spec:
  containers:
  - name: my-container
    image: nginx
`

	unstructuredResource, err := YamlToUnstructured([]byte(rawYAML))

	assert.Nil(t, err)
	assert.NotNil(t, unstructuredResource)
	assert.Equal(t, "{{ .Chart.Name }}", unstructuredResource.GetLabels()["app"])
}
