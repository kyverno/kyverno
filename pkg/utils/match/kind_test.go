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

func Test_CheckKind_PolicyExceptionSubresources(t *testing.T) {
	// Test that parent resource in exception matches subresources in policy
	// This is the main fix for issue #13086
	// Note: Using allowEphemeralContainers=true because that's how exceptions call this function

	// PolicyException specifies "Pod", policy matches on "Pod/exec" - should match
	match := CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, true, "Parent resource 'Pod' should match subresource 'exec'")

	// PolicyException specifies "Pod", policy matches on "Pod/log" - should match
	match = CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "log", true)
	assert.Equal(t, match, true, "Parent resource 'Pod' should match subresource 'log'")

	// PolicyException specifies "Pod", policy matches on "Pod/attach" - should match
	match = CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "attach", true)
	assert.Equal(t, match, true, "Parent resource 'Pod' should match subresource 'attach'")

	// PolicyException specifies "Pod", policy matches on "Pod/portforward" - should match
	match = CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "portforward", true)
	assert.Equal(t, match, true, "Parent resource 'Pod' should match subresource 'portforward'")

	// PolicyException specifies "v1/Pod", policy matches on "Pod/exec" - should match
	match = CheckKind([]string{"v1/Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, true, "Versioned parent resource 'v1/Pod' should match subresource 'exec'")

	// PolicyException specifies "Deployment", policy matches on "Deployment/scale" - should match
	match = CheckKind([]string{"Deployment"}, schema.GroupVersionKind{Kind: "Deployment", Group: "apps", Version: "v1"}, "scale", true)
	assert.Equal(t, match, true, "Parent resource 'Deployment' should match subresource 'scale'")

	// PolicyException specifies "apps/v1/Deployment", policy matches on "Deployment/scale" - should match
	match = CheckKind([]string{"apps/v1/Deployment"}, schema.GroupVersionKind{Kind: "Deployment", Group: "apps", Version: "v1"}, "scale", true)
	assert.Equal(t, match, true, "Fully qualified parent resource should match subresource 'scale'")

	// PolicyException specifies "Service", policy matches on "Pod/exec" - should NOT match
	match = CheckKind([]string{"Service"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, false, "Different resource kind should not match")

	// PolicyException specifies exact subresource "Pod/log", policy matches on "Pod/exec" - should NOT match
	match = CheckKind([]string{"Pod/log"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, false, "Different subresources should not match")

	// PolicyException specifies wildcard "*", policy matches on "Pod/exec" - should match
	match = CheckKind([]string{"*"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, true, "Wildcard should match any subresource")

	// Multiple kinds in exception, one matches parent to subresource
	match = CheckKind([]string{"Service", "Pod", "ConfigMap"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", true)
	assert.Equal(t, match, true, "Should match when one of multiple kinds matches")

	// Test with allowEphemeralContainers=false to ensure backward compatibility
	match = CheckKind([]string{"Pod"}, schema.GroupVersionKind{Kind: "Pod", Group: "", Version: "v1"}, "exec", false)
	assert.Equal(t, match, false, "With allowEphemeralContainers=false, parent should NOT match subresource")
}
