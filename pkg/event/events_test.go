package event

import (
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
)

func TestMessageLength(t *testing.T) {
	msg := "policy psa/baseline fail: Validation rule 'baseline' failed. It violates PodSecurity \"restricted:latest\": (Forbidden reason: allowPrivilegeEscalation != false, field error list: [spec.containers[0].securityContext.allowPrivilegeEscalation is forbidden, forbidden values found: nil])(Forbidden reason: unrestricted capabilities, field error list: [spec.containers[0].securityContext.capabilities.drop: Required value])(Forbidden reason: host namespaces, field error list: [spec.hostNetwork is forbidden, forbidden values found: true])(Forbidden reason: hostPath volumes, field error list: [spec.volumes[1].hostPath is forbidden, forbidden values found: /run/xtables.lock, spec.volumes[2].hostPath is forbidden, forbidden values found: /lib/modules])(Forbidden reason: privileged, field error list: [spec.containers[0].securityContext.privileged is forbidden, forbidden values found: true])(Forbidden reason: restricted volume types, field error list: [spec.volumes[1].hostPath: Forbidden, spec.volumes[2].hostPath: Forbidden])(Forbidden reason: runAsNonRoot != true, field error list: [spec.containers[0].securityContext.runAsNonRoot: Required value])(Forbidden reason: seccompProfile, field error list: [spec.containers[0].securityContext.seccompProfile.type: Required value])"
	assert.Assert(t, len(msg) > 1024)

	resp := engineapi.NewRuleResponse("podSecurity", engineapi.Validation, msg, engineapi.RuleStatusFail)

	resource := &engineapi.ResourceSpec{
		Kind:       "Pod",
		APIVersion: "v1",
		Namespace:  "default",
		UID:        "9005aec3-f779-4d19-985b-3ff51a695cca",
	}

	eventMsg := buildPolicyEventMessage(*resp, *resource, true)
	assert.Equal(t, 1024, len(eventMsg))
}
