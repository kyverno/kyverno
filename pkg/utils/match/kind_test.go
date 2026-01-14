package match

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_CheckKind(t *testing.T) {
	match := CheckKind([]string{"*"}, schema.GroupVersionKind{Kind: "Deployment", Group: "", Version: "v1"}, "", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"v1/Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"tekton.dev/v1beta1/TaskRun"}, schema.GroupVersionKind{Kind: "TaskRun", Group: "tekton.dev", Version: "v1beta1"}, "", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"tekton.dev/*/TaskRun"}, schema.GroupVersionKind{Kind: "TaskRun", Group: "tekton.dev", Version: "v1alpha1"}, "", false)
	assert.Equal(t, match, true)

	// Though both 'pods', 'pods/status' have same kind i.e. 'Pod' but they are different resources, 'subresourceInAdmnReview' is used in determining that.
	match = CheckKind([]string{"v1/Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "status", false)
	assert.Equal(t, match, false)

	// Though both 'pods', 'pods/ephemeralcontainers' have same kind i.e. 'Pod' but they are different resources, allowEphemeralContainers governs how to match this case.
	match = CheckKind([]string{"v1/Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "ephemeralcontainers", true)
	assert.Equal(t, match, true)

	// Though both 'pods', 'pods/ephemeralcontainers' have same kind i.e. 'Pod' but they are different resources, allowEphemeralContainers governs how to match this case.
	match = CheckKind([]string{"v1/Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "ephemeralcontainers", false)
	assert.Equal(t, match, false)

	match = CheckKind([]string{"postgresdb"}, schema.GroupVersionKind{Kind: "postgresdb", Group: "acid.zalan.do", Version: "v1"}, "", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"Postgresdb"}, schema.GroupVersionKind{Kind: "postgresdb", Group: "acid.zalan.do", Version: "v1"}, "", false)
	assert.Equal(t, match, false)

	match = CheckKind([]string{"networking.k8s.io/v1/NetworkPolicy/status"}, schema.GroupVersionKind{Kind: "NetworkPolicy", Group: "networking.k8s.io", Version: "v1"}, "status", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"v1/Pod.status"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "status", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"*/Pod.eviction"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "eviction", false)
	assert.Equal(t, match, true)

	match = CheckKind([]string{"v1alpha1/Pod.eviction"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "eviction", false)
	assert.Equal(t, match, false)
}
