package response

import (
	"testing"

	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
)

var sourceYAML = `
policy:
  name: disallow-bind-mounts
resource:
  kind: Pod
  apiVersion: v1
  name: image-with-hostpath
rules:
- name: validate-hostPath
  type: Validation
  status: fail
`

func Test_parse_yaml(t *testing.T) {
	var pr PolicyResponse
	if err := yaml.Unmarshal([]byte(sourceYAML), &pr); err != nil {
		t.Errorf("failed to parse YAML: %v", err)
		return
	}
	assert.Equal(t, 1, len(pr.Rules))
	assert.Equal(t, RuleStatusFail, pr.Rules[0].Status)
}
