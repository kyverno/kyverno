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

	apiVersion, kind = GetKindFromGVK("*.*")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "*/*", kind)

	apiVersion, kind = GetKindFromGVK("*/*")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "*/*", kind)

	apiVersion, kind = GetKindFromGVK("Pod")
	assert.Equal(t, "", apiVersion)
	assert.Equal(t, "Pod", kind)

	apiVersion, kind = GetKindFromGVK("v1/Pod")
	assert.Equal(t, "v1", apiVersion)
	assert.Equal(t, "Pod", kind)

	apiVersion, kind = GetKindFromGVK("batch/*/CronJob")
	assert.Equal(t, "batch/*", apiVersion)
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

	apiVersion, kind = GetKindFromGVK("apps/v1/Deployment/Scale")
	assert.Equal(t, "apps/v1", apiVersion)
	assert.Equal(t, "Deployment/Scale", kind)

	apiVersion, kind = GetKindFromGVK("v1/ReplicationController/Scale")
	assert.Equal(t, "v1", apiVersion)
	assert.Equal(t, "ReplicationController/Scale", kind)

	apiVersion, kind = GetKindFromGVK("*/ReplicationController/Scale")
	assert.Equal(t, "*", apiVersion)
	assert.Equal(t, "ReplicationController/Scale", kind)

	apiVersion, kind = GetKindFromGVK("*/Deployment/scale")
	assert.Equal(t, "*", apiVersion)
	assert.Equal(t, "Deployment/scale", kind)

	apiVersion, kind = GetKindFromGVK("*/Deployment.scale")
	assert.Equal(t, "*", apiVersion)
	assert.Equal(t, "Deployment/scale", kind)

	apiVersion, kind = GetKindFromGVK("*/Deployment/scale")
	assert.Equal(t, "*", apiVersion)
	assert.Equal(t, "Deployment/scale", kind)

	apiVersion, kind = GetKindFromGVK("apps/v1/Deployment.scale")
	assert.Equal(t, "apps/v1", apiVersion)
	assert.Equal(t, "Deployment/scale", kind)
}

func Test_SplitSubresource(t *testing.T) {
	var kind, subresource string
	kind, subresource = SplitSubresource("TaskRun/Status")
	assert.Equal(t, kind, "TaskRun")
	assert.Equal(t, subresource, "Status")

	kind, subresource = SplitSubresource("TaskRun/status")
	assert.Equal(t, kind, "TaskRun")
	assert.Equal(t, subresource, "status")
}

func Test_GroupVersionMatches(t *testing.T) {
	groupVersion, serverResourceGroupVersion := "v1", "v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), true)

	// If user does not specify a group, then it is considered as legacy group which is empty
	groupVersion, serverResourceGroupVersion = "v1", "networking.k8s.io/v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), false)

	groupVersion, serverResourceGroupVersion = "*", "v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), true)

	groupVersion, serverResourceGroupVersion = "certificates.k8s.io/*", "certificates.k8s.io/v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), true)

	groupVersion, serverResourceGroupVersion = "*", "certificates.k8s.io/v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), true)

	groupVersion, serverResourceGroupVersion = "certificates.k8s.io/*", "networking.k8s.io/v1"
	assert.Equal(t, GroupVersionMatches(groupVersion, serverResourceGroupVersion), false)
}

func TestParseKindSelector(t *testing.T) {
	type args struct {
		input string
	}
	type want struct {
		group       string
		version     string
		kind        string
		subresource string
	}
	tests := []struct {
		name string
		args args
		want want
	}{{
		args: args{"*"},
		want: want{"*", "*", "*", ""},
	}, {
		args: args{"*.*"},
		want: want{"*", "*", "*", "*"},
	}, {
		args: args{"*/*"},
		want: want{"*", "*", "*", "*"},
	}, {
		args: args{"Pod"},
		want: want{"*", "*", "Pod", ""},
	}, {
		args: args{"v1/Pod"},
		want: want{"*", "v1", "Pod", ""},
	}, {
		args: args{"batch/*/CronJob"},
		want: want{"batch", "*", "CronJob", ""},
	}, {
		args: args{"storage.k8s.io/v1/CSIDriver"},
		want: want{"storage.k8s.io", "v1", "CSIDriver", ""},
	}, {
		args: args{"tekton.dev/v1beta1/TaskRun/status"},
		want: want{"tekton.dev", "v1beta1", "TaskRun", "status"},
	}, {
		args: args{"v1/Pod.status"},
		want: want{"*", "v1", "Pod", "status"},
	}, {
		args: args{"v1/Pod/status"},
		want: want{"*", "v1", "Pod", "status"},
	}, {
		args: args{"Pod.status"},
		want: want{"*", "*", "Pod", "status"},
	}, {
		args: args{"Pod/status"},
		want: want{"*", "*", "Pod", "status"},
	}, {
		args: args{"apps/v1/Deployment/scale"},
		want: want{"apps", "v1", "Deployment", "scale"},
	}, {
		args: args{"v1/ReplicationController/scale"},
		want: want{"*", "v1", "ReplicationController", "scale"},
	}, {
		args: args{"*/ReplicationController/scale"},
		want: want{"*", "*", "ReplicationController", "scale"},
	}, {
		args: args{"*/Deployment/scale"},
		want: want{"*", "*", "Deployment", "scale"},
	}, {
		args: args{"*/Deployment.scale"},
		want: want{"*", "*", "Deployment", "scale"},
	}, {
		args: args{"apps/v1/Deployment.scale"},
		want: want{"apps", "v1", "Deployment", "scale"},
	}, {
		args: args{"*/scale"},
		want: want{"*", "*", "*", "scale"},
	}, {
		args: args{"Pod/*"},
		want: want{"*", "*", "Pod", "*"},
	}, {
		args: args{"*/*/*"},
		want: want{"*", "*", "*", "*"},
	}, {
		args: args{"*/*/*/*"},
		want: want{"*", "*", "*", "*"},
	}, {
		args: args{"*/*/*/*/*"},
		want: want{"", "", "", ""},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, version, kind, subresource := ParseKindSelector(tt.args.input)
			if group != tt.want.group {
				t.Errorf("ParseKindSelector() group = %v, want %v", group, tt.want.group)
			}
			if version != tt.want.version {
				t.Errorf("ParseKindSelector() version = %v, want %v", version, tt.want.version)
			}
			if kind != tt.want.kind {
				t.Errorf("ParseKindSelector() kind = %v, want %v", kind, tt.want.kind)
			}
			if subresource != tt.want.subresource {
				t.Errorf("ParseKindSelector() subresource = %v, want %v", subresource, tt.want.subresource)
			}
		})
	}
}
