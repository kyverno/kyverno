package mutate

import (
	"testing"
	"regexp"
	"strings"
	"gotest.tools/assert"
	assertnew "github.com/stretchr/testify/assert"
)

func Test_preProcessStrategicMergePatch(t *testing.T){
	rawPolicy := []byte(`{"metadata":{"+(annotations)":{"+(annotation1)":"atest1"},"labels":{"+(label1)":"test1"}},"spec":{"(volumes)":[{"(hostPath)":{"path":"/var/run/docker.sock"}}],"containers":[{"(image)":"*:latest","command":["ls"],"imagePullPolicy":"Always"}]}}`)

	rawResource := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"annotations":{"annotation1":"atest2"},"labels":{"label1":"test2","label2":"test2"},"name":"check-root-user"},"spec":{"containers":[{"command":["ll"],"image":"nginx:latest","imagePullPolicy":"Never","name":"nginx"},{"image":"busybox:latest","imagePullPolicy":"Never","name":"busybox"}],"volumes":[{"hostPath":{"path":"/var/run/docker.sock"},"name":"test-volume"}]}}`)
	
	expected := `{"metadata": {"annotations": {"annotation1": "atest1"}, "labels": {"label1": "test1"}},"spec": {"containers": [{"command": ["ls", "ll"], "imagePullPolicy": "Always", "name": "nginx"},{"command": ["ls"], "imagePullPolicy": "Always", "name": "busybox"}]}}`

	preProcessedPolicy, err := preProcessStrategicMergePatch(string(rawPolicy), string(rawResource))
	assert.NilError(t, err)
	output, err := preProcessedPolicy.String()
	assert.NilError(t, err)
	re := regexp.MustCompile("\\n")
	if !assertnew.Equal(t, strings.ReplaceAll(expected, " ", ""), strings.ReplaceAll(re.ReplaceAllString(output, ""), " ","")) {
		t.FailNow()
	}
}