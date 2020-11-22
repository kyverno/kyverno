package apply

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
)

func Test_ApplyCommandPass(t *testing.T) {
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"../../../samples/best_practices/disallow_latest_tag.yaml", "-r", "../../../test/resources/pod_with_version_tag.yaml", "--policy-report"})
	cmd.Execute()
	actual, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	expected := `
	applying 1 policy to 1 resource...
	----------------------------------------------------------------------
	POLICY REPORT:
	----------------------------------------------------------------------
	apiVersion: wgpolicyk8s.io/v1alpha1
	kind: ClusterPolicyReport
	metadata:
	  name: clusterpolicyreport
	results:
	- message: Validation rule 'require-image-tag' succeeded.
	  policy: disallow-latest-tag
	  resources:
	  - apiVersion: v1
		kind: Pod
		name: myapp-pod
		namespace: default
	  rule: require-image-tag
	  scored: true
	  status: pass
	- message: Validation rule 'validate-image-tag' succeeded.
	  policy: disallow-latest-tag
	  resources:
	  - apiVersion: v1
		kind: Pod
		name: myapp-pod
		namespace: default
	  rule: validate-image-tag
	  scored: true
	  status: pass
	summary:
	  error: 0
	  fail: 0
	  pass: 2
	  skip: 0
	  warn: 0
	`
	actualOutput := strings.Replace(strings.Replace(strings.Replace(string(actual), " ", "", -1), "\t", "", -1), "\n", "", -1)
	expectedOutput := strings.Replace(strings.Replace(strings.Replace(expected, " ", "", -1), "\t", "", -1), "\n", "", -1)
	if expectedOutput != actualOutput {
		t.Fatalf("expected:\"%s\" \ngot: \"%s\"", expectedOutput, actualOutput)
	}
}

func Test_ApplyCommandFail(t *testing.T) {
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"../../../samples/best_practices/require_pod_requests_limits.yaml", "-r", "../../../test/resources/pod_with_latest_tag.yaml", "--policy-report"})
	cmd.Execute()
	actual, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	expected := `
	applying 1 policy to 1 resource... 
	----------------------------------------------------------------------
	POLICY REPORT:
	----------------------------------------------------------------------
	apiVersion: wgpolicyk8s.io/v1alpha1
	kind: ClusterPolicyReport
	metadata:
	name: clusterpolicyreport
	results:
	- message: 'Validation error: CPU and memory resource requests and limits are required; Validation rule validate-resources failed at path /spec/containers/0/resources/limits/'
	policy: require-pod-requests-limits
	resources:
	- apiVersion: v1
		kind: Pod
		name: myapp-pod
		namespace: default
	rule: validate-resources
	scored: true
	status: fail
	summary:
	error: 0
	fail: 1
	pass: 0
	skip: 0
	warn: 0
	`
	actualOutput := strings.Replace(strings.Replace(strings.Replace(string(actual), " ", "", -1), "\t", "", -1), "\n", "", -1)
	expectedOutput := strings.Replace(strings.Replace(strings.Replace(expected, " ", "", -1), "\t", "", -1), "\n", "", -1)
	if expectedOutput != actualOutput {
		t.Fatalf("expected: \"%s\" \ngot: \"%s\"", expectedOutput, actualOutput)
	}
}
