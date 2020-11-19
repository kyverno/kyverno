package apply

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"
)

func Test_ExecuteCommand(t *testing.T) {
	cmd := Command()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetArgs([]string{"../../../samples/best_practices/disallow_latest_tag.yaml", "-r", "../../../test/resources/pod_with_version_tag.yaml", "--policy-report"})
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	a := `
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
	actualOutput := strings.Replace(strings.Replace(strings.Replace(string(out), " ", "", -1), "\t", "", -1), "\n", "", -1)
	expectedOutput := strings.Replace(strings.Replace(strings.Replace(a, " ", "", -1), "\t", "", -1), "\n", "", -1)
	if expectedOutput != actualOutput {
		t.Fatalf("expected \"%s\" got \"%s\"", expectedOutput, actualOutput)
	}
}
