package pss

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

// hostProbePod mirrors the workloads users reported in #15111: a container whose liveness probe pins
// an explicit host, as the EKS Pod Identity agent and the csi-driver-nfs controller both do.
var hostProbePod = []byte(`{
	"kind": "Pod",
	"apiVersion": "v1",
	"metadata": {"name": "eks-pod-identity-agent", "namespace": "kube-system"},
	"spec": {
		"containers": [{
			"name": "eks-pod-identity-agent",
			"image": "602401143452.dkr.ecr.eu-west-1.amazonaws.com/eks/eks-pod-identity-agent:0.1.0",
			"livenessProbe": {"httpGet": {"host": "localhost", "path": "/healthz", "port": 2703}}
		}]
	}
}`)

// Test_HostProbes_VersionGating is the regression test for #14521: the Host Probes / Lifecycle Hooks
// control only exists in the Pod Security Standards from v1.34, so pinning podSecurity.version to an
// earlier release must not enforce it. Leaving the version unset keeps enforcing it, which is what
// almost every policy does and what keeps the default strict.
func Test_HostProbes_VersionGating(t *testing.T) {
	tests := []struct {
		version     string
		wantAllowed bool
	}{
		{"", false},
		{"latest", false},
		{"v1.35", false},
		{"v1.34", false},
		{"v1.33", true},
		{"v1.29", true},
	}

	for _, test := range tests {
		var pod corev1.Pod
		err := json.Unmarshal(hostProbePod, &pod)
		assert.NilError(t, err)

		levelVersion, err := ParseVersion(api.LevelBaseline, test.version)
		assert.NilError(t, err)

		allowed, checks := EvaluatePod(levelVersion, nil, &pod)
		assert.Equal(t, allowed, test.wantAllowed,
			"version %q: expected allowed=%v, got %v (%s)", test.version, test.wantAllowed, allowed, FormatChecksPrint(checks))
	}
}

// Test_HostProbes_ControlName covers the control being addressable by name at all. Without the
// mapping the check reported an empty control name, so it could neither be read in a report nor
// named in an exclusion (#15111).
func Test_HostProbes_ControlName(t *testing.T) {
	assert.Equal(t, pssutils.PSSControlIDToName("hostProbesAndHostLifecycle"), "Host Probes / Lifecycle Hooks")
	assert.Assert(t, len(pssutils.PSS_control_name_to_ids["Host Probes / Lifecycle Hooks"]) == 1)
	assert.Equal(t, pssutils.PSS_control_name_to_ids["Host Probes / Lifecycle Hooks"][0], "hostProbesAndHostLifecycle")
}

// Test_HostProbes_RequiresImages checks the control is treated as container level, like the other
// controls that restrict fields under spec.containers[*]. Excluding it without images would silently
// match no container, so the policy is rejected with a message that says what to add.
func Test_HostProbes_RequiresImages(t *testing.T) {
	path := field.NewPath("spec", "rules[0]", "validate", "podSecurity", "exclude[0]")

	withoutImages := kyvernov1.PodSecurityStandard{ControlName: "Host Probes / Lifecycle Hooks"}
	errs := withoutImages.Validate(path)
	assert.Assert(t, len(errs) == 1, "expected the exclusion to be rejected without images, got %v", errs)

	withImages := kyvernov1.PodSecurityStandard{
		ControlName: "Host Probes / Lifecycle Hooks",
		Images:      []string{"602401143452.dkr.ecr.*.amazonaws.com/eks/eks-pod-identity-agent:*"},
	}
	assert.Assert(t, len(withImages.Validate(path)) == 0)
}

// Test_AllPodSecurityChecksHaveAControlName guards the gap that let this slip through: the Host Probes
// check was vendored in with the 1.34 standards but never mapped to a control name, so it could not be
// excluded and showed up with an empty name in reports. Any future control added upstream fails here
// until it is mapped.
func Test_AllPodSecurityChecksHaveAControlName(t *testing.T) {
	for _, check := range policy.DefaultChecks() {
		name := pssutils.PSSControlIDToName(string(check.ID))
		assert.Assert(t, name != "",
			"pod security check %q has no control name: add it to PSS_control_name_to_ids and to the controlName enum", check.ID)
	}
}
