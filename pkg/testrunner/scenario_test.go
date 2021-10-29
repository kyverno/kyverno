package testrunner

import (
	"io/ioutil"
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/response"
	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
)

var sourceYAML = `
input:
  policy: test/best_practices/disallow_bind_mounts.yaml
  resource: test/resources/disallow_host_filesystem.yaml
expected:
  validation:
    policyresponse:
      policy:
        namespace: ''
        name: disallow-bind-mounts
      resource:
        kind: Pod
        apiVersion: v1
        namespace: ''
        name: image-with-hostpath
      rules:
        - name: validate-hostPath
          type: Validation
          status: fail
`

func Test_parse_yaml(t *testing.T) {
	var s TestCase
	if err := yaml.Unmarshal([]byte(sourceYAML), &s); err != nil {
		t.Errorf("failed to parse YAML: %v", err)
		return
	}

	assert.Equal(t, s.Expected.Validation.PolicyResponse.Policy.Name, "disallow-bind-mounts")
	assert.Equal(t, 1, len(s.Expected.Validation.PolicyResponse.Rules), "invalid rule count")
	assert.Equal(t, response.RuleStatusFail, s.Expected.Validation.PolicyResponse.Rules[0].Status, "invalid status")
}

func Test_parse_file(t *testing.T) {
	s, err := loadScenario(t, "test/scenarios/samples/best_practices/disallow_bind_mounts_fail.yaml")
	assert.NilError(t, err)

	assert.Equal(t, 1, len(s.TestCases))
	assert.Equal(t, s.TestCases[0].Expected.Validation.PolicyResponse.Policy.Name, "disallow-bind-mounts")
	assert.Equal(t, 1, len(s.TestCases[0].Expected.Validation.PolicyResponse.Rules), "invalid rule count")
	assert.Equal(t, response.RuleStatusFail, s.TestCases[0].Expected.Validation.PolicyResponse.Rules[0].Status, "invalid status")
}

func Test_parse_file2(t *testing.T) {
	path := getRelativePath("test/scenarios/samples/best_practices/disallow_bind_mounts_fail.yaml")
	data, err := ioutil.ReadFile(path)
	assert.NilError(t, err)

	strData := string(data)
	var s TestCase
	if err := yaml.Unmarshal([]byte(strData), &s); err != nil {
		t.Errorf("failed to parse YAML: %v", err)
		return
	}

	assert.Equal(t, s.Expected.Validation.PolicyResponse.Policy.Name, "disallow-bind-mounts")
	assert.Equal(t, 1, len(s.Expected.Validation.PolicyResponse.Rules), "invalid rule count")
	assert.Equal(t, response.RuleStatusFail, s.Expected.Validation.PolicyResponse.Rules[0].Status, "invalid status")
}
