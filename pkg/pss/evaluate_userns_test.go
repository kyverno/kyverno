package pss

import (
	"sort"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

func Test_EvaluatePod_UserNamespaces(t *testing.T) {
	unmasked := corev1.UnmaskedProcMount
	newPod := func(hostUsers *bool, sc *corev1.SecurityContext) *corev1.Pod {
		return &corev1.Pod{Spec: corev1.PodSpec{
			HostUsers: hostUsers,
			SecurityContext: &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx", SecurityContext: sc}},
		}}
	}
	// passes restricted except for the fields under test
	restrictedSC := func() *corev1.SecurityContext {
		return &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		}
	}
	unmaskedSC := restrictedSC()
	unmaskedSC.ProcMount = &unmasked
	rootSC := restrictedSC()
	rootSC.RunAsNonRoot = nil
	rootSC.RunAsUser = ptr.To[int64](0)
	nonRootFalseSC := restrictedSC()
	nonRootFalseSC.RunAsNonRoot = ptr.To(false)
	privilegedSC := restrictedSC()
	privilegedSC.Privileged = ptr.To(true)
	excludeProcMount := []kyvernov1.PodSecurityStandard{{ControlName: "/proc Mount Type", Images: []string{"nginx"}}}
	excludeUnmasked := []kyvernov1.PodSecurityStandard{{
		ControlName:     "/proc Mount Type",
		Images:          []string{"nginx"},
		RestrictedField: "spec.containers[*].securityContext.procMount",
		Values:          []string{"Unmasked"},
	}}

	tests := []struct {
		name      string
		level     api.Level
		version   string
		pod       *corev1.Pod
		excludes  []kyvernov1.PodSecurityStandard
		allowed   bool
		failedIDs []string
	}{
		// baseline procMount
		{"baseline latest, userns, unmasked", api.LevelBaseline, "latest", newPod(ptr.To(false), unmaskedSC), nil, true, nil},
		{"baseline latest, no userns, unmasked", api.LevelBaseline, "latest", newPod(ptr.To(true), unmaskedSC), nil, false, []string{"procMount"}},
		{"baseline latest, hostUsers unset, unmasked", api.LevelBaseline, "latest", newPod(nil, unmaskedSC), nil, false, []string{"procMount"}},
		{"baseline v1.32, userns, unmasked", api.LevelBaseline, "v1.32", newPod(ptr.To(false), unmaskedSC), nil, false, []string{"procMount"}},
		// restricted procMount
		{"restricted latest, userns, unmasked", api.LevelRestricted, "latest", newPod(ptr.To(false), unmaskedSC), nil, false, []string{"procMount_restricted"}},
		{"restricted latest, no userns, unmasked", api.LevelRestricted, "latest", newPod(ptr.To(true), unmaskedSC), nil, false, []string{"procMount", "procMount_restricted"}},
		// evaluatePSS runs checks newer than a pinned version, so procMount_restricted also reports here
		{"restricted v1.32, userns, unmasked", api.LevelRestricted, "v1.32", newPod(ptr.To(false), unmaskedSC), nil, false, []string{"procMount", "procMount_restricted"}},
		// restricted runAsNonRoot and runAsUser
		{"restricted latest, userns, runAsUser 0", api.LevelRestricted, "latest", newPod(ptr.To(false), rootSC), nil, true, nil},
		{"restricted latest, no userns, runAsUser 0", api.LevelRestricted, "latest", newPod(ptr.To(true), rootSC), nil, false, []string{"runAsNonRoot", "runAsUser"}},
		{"restricted latest, hostUsers unset, runAsUser 0", api.LevelRestricted, "latest", newPod(nil, rootSC), nil, false, []string{"runAsNonRoot", "runAsUser"}},
		{"restricted v1.32, userns, runAsUser 0", api.LevelRestricted, "v1.32", newPod(ptr.To(false), rootSC), nil, false, []string{"runAsNonRoot", "runAsUser"}},
		{"restricted latest, userns, runAsNonRoot false", api.LevelRestricted, "latest", newPod(ptr.To(false), nonRootFalseSC), nil, true, nil},
		{"restricted v1.32, userns, runAsNonRoot false", api.LevelRestricted, "v1.32", newPod(ptr.To(false), nonRootFalseSC), nil, false, []string{"runAsNonRoot"}},
		// other controls are not relaxed
		{"restricted latest, userns, privileged", api.LevelRestricted, "latest", newPod(ptr.To(false), privilegedSC), nil, false, []string{"privileged"}},
		// exclusions cover both procMount checks
		{"restricted latest, userns, unmasked, control excluded", api.LevelRestricted, "latest", newPod(ptr.To(false), unmaskedSC), excludeProcMount, true, nil},
		{"restricted v1.32, no userns, unmasked, control excluded", api.LevelRestricted, "v1.32", newPod(ptr.To(true), unmaskedSC), excludeProcMount, true, nil},
		{"restricted latest, userns, unmasked, value excluded", api.LevelRestricted, "latest", newPod(ptr.To(false), unmaskedSC), excludeUnmasked, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levelVersion, err := ParseVersion(tt.level, tt.version)
			assert.NilError(t, err)
			allowed, results := EvaluatePod(levelVersion, tt.excludes, tt.pod)
			var ids []string
			for _, r := range results {
				ids = append(ids, r.ID)
			}
			sort.Strings(ids)
			assert.Equal(t, allowed, tt.allowed, "failed checks: %v", ids)
			assert.DeepEqual(t, ids, tt.failedIDs)
		})
	}
}
