package event

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_buildPolicyEventMessage_with_namespaced_resource(t *testing.T) {
	resp := engineapi.NewRuleResponse("require-team-label", engineapi.Validation, "missing required label: team", engineapi.RuleStatusFail, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Pod",
		Namespace: "prod",
		Name:      "nginx",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "Pod prod/nginx")
	assert.Contains(t, msg, "[require-team-label]")
	assert.Contains(t, msg, "fail")
	assert.Contains(t, msg, "missing required label: team")
	assert.NotContains(t, msg, "(blocked)")
}

func Test_buildPolicyEventMessage_cluster_scoped(t *testing.T) {
	resp := engineapi.NewRuleResponse("ns-labels", engineapi.Validation, "namespace needs annotations", engineapi.RuleStatusFail, nil)
	res := engineapi.ResourceSpec{
		Kind: "Namespace",
		Name: "dev",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "Namespace dev")
	assert.NotContains(t, msg, "Namespace /dev") // no double slash
}

func Test_buildPolicyEventMessage_blocked(t *testing.T) {
	resp := engineapi.NewRuleResponse("no-privileged", engineapi.Validation, "privileged containers not allowed", engineapi.RuleStatusFail, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "hacker-pod",
	}

	msg := buildPolicyEventMessage(*resp, res, true)

	assert.Contains(t, msg, "(blocked)")
}

func Test_buildPolicyEventMessage_no_rule_name(t *testing.T) {
	resp := engineapi.NewRuleResponse("", engineapi.Validation, "", engineapi.RuleStatusPass, nil)
	res := engineapi.ResourceSpec{
		Kind:      "ConfigMap",
		Namespace: "default",
		Name:      "settings",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "ConfigMap default/settings")
	assert.Contains(t, msg, "pass")
}

func Test_buildPolicyEventMessage_pass_status(t *testing.T) {
	resp := engineapi.NewRuleResponse("validate-config", engineapi.Validation, "looks good", engineapi.RuleStatusPass, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Service",
		Namespace: "kube-system",
		Name:      "coredns",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "pass")
	assert.Contains(t, msg, "Service kube-system/coredns")
}

func Test_buildPolicyEventMessage_skip_status(t *testing.T) {
	resp := engineapi.NewRuleResponse("check-env", engineapi.Validation, "precondition not met", engineapi.RuleStatusSkip, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Deployment",
		Namespace: "staging",
		Name:      "api",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "skip")
	assert.Contains(t, msg, "Deployment staging/api")
}

func Test_buildPolicyEventMessage_error_status(t *testing.T) {
	resp := engineapi.NewRuleResponse("verify-sig", engineapi.ImageVerify, "cosign verification failed", engineapi.RuleStatusError, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "app",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "error")
	assert.Contains(t, msg, "cosign verification failed")
}

func Test_buildPolicyEventMessage_mutation(t *testing.T) {
	resp := engineapi.NewRuleResponse("inject-sidecar", engineapi.Mutation, "added istio-proxy container", engineapi.RuleStatusPass, nil)
	res := engineapi.ResourceSpec{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "backend",
	}

	msg := buildPolicyEventMessage(*resp, res, false)

	assert.Contains(t, msg, "[inject-sidecar]")
	assert.Contains(t, msg, "added istio-proxy container")
}

func Test_NewResourceGenerationEvent(t *testing.T) {
	res := kyvernov1.ResourceSpec{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Namespace:  "default",
		Name:       "app-config",
		UID:        "abc123",
	}

	ev := NewResourceGenerationEvent("config-generator", "gen-cm", GeneratePolicyController, res)

	assert.Equal(t, "ConfigMap", ev.Regarding.Kind)
	assert.Equal(t, "default", ev.Regarding.Namespace)
	assert.Equal(t, "app-config", ev.Regarding.Name)
	assert.Equal(t, PolicyApplied, ev.Reason)
	assert.Equal(t, GeneratePolicyController, ev.Source)
	assert.Contains(t, ev.Message, "Created ConfigMap app-config")
	assert.Contains(t, ev.Message, "config-generator/gen-cm")
}

func Test_NewResourceGenerationEvent_cluster_resource(t *testing.T) {
	res := kyvernov1.ResourceSpec{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "tenant-a",
	}

	ev := NewResourceGenerationEvent("ns-creator", "make-ns", GeneratePolicyController, res)

	assert.Equal(t, "Namespace", ev.Regarding.Kind)
	assert.Empty(t, ev.Regarding.Namespace)
	assert.Equal(t, "tenant-a", ev.Regarding.Name)
}

func Test_NewFailedEvent_with_rule(t *testing.T) {
	res := kyvernov1.ResourceSpec{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "prod",
		Name:       "frontend",
		UID:        "xyz789",
	}
	err := errors.New("resource limits missing")

	ev := NewFailedEvent(err, "enforce-limits", "check-cpu", AdmissionController, res)

	assert.Equal(t, "Deployment", ev.Regarding.Kind)
	assert.Equal(t, "prod", ev.Regarding.Namespace)
	assert.Equal(t, "frontend", ev.Regarding.Name)
	assert.Equal(t, PolicyError, ev.Reason)
	assert.Equal(t, AdmissionController, ev.Source)
	assert.Equal(t, None, ev.Action)
	assert.Contains(t, ev.Message, "enforce-limits/check-cpu")
	assert.Contains(t, ev.Message, "resource limits missing")
}

func Test_NewFailedEvent_no_rule(t *testing.T) {
	res := kyvernov1.ResourceSpec{
		Kind:      "Pod",
		Namespace: "default",
		Name:      "busybox",
	}
	err := errors.New("something went wrong")

	ev := NewFailedEvent(err, "baseline-policy", "", PolicyController, res)

	assert.Contains(t, ev.Message, "policy baseline-policy error")
	assert.NotContains(t, ev.Message, "baseline-policy/")
}

func Test_NewCleanupPolicyEvent_success(t *testing.T) {
	pol := &kyvernov2.ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "remove-stale-pods",
			UID:  "pol-uid-1",
		},
	}
	res := unstructured.Unstructured{}
	res.SetAPIVersion("v1")
	res.SetKind("Pod")
	res.SetNamespace("default")
	res.SetName("zombie")

	ev := NewCleanupPolicyEvent(pol, res, nil)

	assert.Equal(t, "remove-stale-pods", ev.Regarding.Name)
	assert.Equal(t, PolicyApplied, ev.Reason)
	assert.Equal(t, ResourceCleanedUp, ev.Action)
	assert.Equal(t, CleanupController, ev.Source)
	assert.Contains(t, ev.Message, "successfully cleaned up")
	assert.Contains(t, ev.Message, "Pod/default/zombie")
}

func Test_NewCleanupPolicyEvent_failure(t *testing.T) {
	pol := &kyvernov2.ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cleanup-cm",
		},
	}
	res := unstructured.Unstructured{}
	res.SetKind("ConfigMap")
	res.SetNamespace("test")
	res.SetName("leftover")
	err := errors.New("forbidden")

	ev := NewCleanupPolicyEvent(pol, res, err)

	assert.Equal(t, PolicyError, ev.Reason)
	assert.Equal(t, None, ev.Action)
	assert.Contains(t, ev.Message, "failed to clean up")
	assert.Contains(t, ev.Message, "forbidden")
}

func Test_NewCleanupPolicyEvent_namespaced(t *testing.T) {
	pol := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-cleaner",
			Namespace: "team-a",
			UID:       "sec-uid",
		},
	}
	res := unstructured.Unstructured{}
	res.SetKind("Secret")
	res.SetNamespace("team-a")
	res.SetName("expired-token")

	ev := NewCleanupPolicyEvent(pol, res, nil)

	assert.Equal(t, "secret-cleaner", ev.Regarding.Name)
	assert.Equal(t, "team-a", ev.Regarding.Namespace)
	assert.Equal(t, ResourceCleanedUp, ev.Action)
}

func Test_resourceKey_namespaced(t *testing.T) {
	res := unstructured.Unstructured{}
	res.SetKind("Deployment")
	res.SetNamespace("prod")
	res.SetName("api-server")

	key := resourceKey(res)

	assert.Equal(t, "Deployment/prod/api-server", key)
}

func Test_resourceKey_cluster_scoped(t *testing.T) {
	res := unstructured.Unstructured{}
	res.SetKind("Namespace")
	res.SetName("monitoring")

	key := resourceKey(res)

	assert.Equal(t, "Namespace/monitoring", key)
}

func Test_resourceKey_no_namespace(t *testing.T) {
	res := unstructured.Unstructured{}
	res.SetKind("ClusterRole")
	res.SetNamespace("") // explicitly empty
	res.SetName("viewer")

	key := resourceKey(res)

	assert.Equal(t, "ClusterRole/viewer", key)
	assert.NotContains(t, key, "//") // shouldn't have double slash
}

func Test_Info_Resource_namespaced(t *testing.T) {
	info := Info{
		Regarding: corev1.ObjectReference{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "web",
		},
	}

	out := info.Resource()

	assert.Equal(t, "Pod/default/web", out)
}

func Test_Info_Resource_cluster_scoped(t *testing.T) {
	info := Info{
		Regarding: corev1.ObjectReference{
			Kind: "Namespace",
			Name: "kube-public",
		},
	}

	out := info.Resource()

	assert.Equal(t, "Namespace/kube-public", out)
}
