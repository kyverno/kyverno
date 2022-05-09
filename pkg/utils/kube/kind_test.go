package kube

import (
	"testing"

	"gotest.tools/assert"
)

func Test_GetKindFromGVK(t *testing.T) {
	var apiVersion, kind string
	apiVersion, kind = GetKindFromGVK("*")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "*", kind)

	apiVersion, kind = GetKindFromGVK("Pod")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "Pod", kind)

	apiVersion, kind = GetKindFromGVK("v1/Pod")
	assert.Equal(t, "v1", apiVersion)
	assert.Equal(t, "Pod", kind)

	apiVersion, kind = GetKindFromGVK("batch/*/CronJob")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "CronJob", kind)

	apiVersion, kind = GetKindFromGVK("storage.k8s.io/v1/CSIDriver")
	assert.Equal(t, "storage.k8s.io/v1", apiVersion)
	assert.Equal(t, "CSIDriver", kind)

	apiVersion, kind = GetKindFromGVK("tekton.dev/v1beta1/TaskRun/Status")
	assert.Equal(t, "tekton.dev/v1beta1", apiVersion)
	assert.Equal(t, "TaskRun/Status", kind)

	apiVersion, kind = GetKindFromGVK("v1/Pod.Status")
	assert.Equal(t, "v1", apiVersion)
	assert.Equal(t, "Pod/Status", kind)

	apiVersion, kind = GetKindFromGVK("Pod.Status")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "Pod/Status", kind)
}

func Test_SplitSubresource(t *testing.T) {
	var kind, subresource string
	kind, subresource = SplitSubresource("TaskRun/Status")
	assert.Equal(t, kind, "TaskRun")
	assert.Equal(t, subresource, "Status")

	kind, subresource = SplitSubresource("TaskRun/status")
	assert.Equal(t, kind, "TaskRun")
	assert.Equal(t, subresource, "status")

	kind, subresource = SplitSubresource("Pod.Status")
	assert.Equal(t, kind, "Pod")
	assert.Equal(t, subresource, "Status")

	kind, subresource = SplitSubresource("v1/Pod/Status")
	assert.Equal(t, kind, "v1/Pod/Status")
	assert.Equal(t, subresource, "")

	kind, subresource = SplitSubresource("v1/Pod.Status")
	assert.Equal(t, kind, "v1/Pod.Status")
	assert.Equal(t, subresource, "")
}
