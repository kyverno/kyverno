package dclient

import (
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_resourceMatches(t *testing.T) {
	ar := metav1.APIResource{Name: "taskruns/status", Kind: "TaskRun"}
	assert.Equal(t, resourceMatches(ar, "TaskRun", "Status"), true)

	ar = metav1.APIResource{Name: "taskruns/status", Kind: "TaskRun"}
	assert.Equal(t, resourceMatches(ar, "TaskRun", ""), false)

	ar = metav1.APIResource{Name: "taskruns", Kind: "TaskRun"}
	assert.Equal(t, resourceMatches(ar, "TaskRun", ""), true)

	ar = metav1.APIResource{Name: "tasks/status", Kind: "Task"}
	assert.Equal(t, resourceMatches(ar, "TaskRun", "Status"), false)
}
