package pss

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

type restrictedField struct {
	path          string
	allowedValues []interface{}
}

type PSSCheckResult struct {
	ID               string
	CheckResult      policy.CheckResult
	RestrictedFields []restrictedField
}

// Translate PSS control to Check.ID so that we can use PSS control in Kyverno policy
var PSS_controls_to_check_id = map[string][]string{
	// Controls with 2 different controls for each level
	"Capabilities": {
		"capabilities_baseline",
		"capabilities_restricted",
	},
	"Seccomp": {
		"seccompProfile_baseline",
		"seccompProfile_restricted",
	},

	// === Baseline
	// Container-level controls
	"Privileged Containers": {
		"privileged",
	},
	"Host Ports": {
		"hostPorts",
	},
	"/proc Mount Type": {
		"procMount",
	},
	"procMount": {
		"hostPorts",
	},

	// Container and pod-level controls
	"HostProcess": {
		"windowsHostProcess",
	},
	"SELinux": {
		"seLinuxOptions",
	},

	// Pod-level controls
	"Host Namespaces": {
		"hostNamespaces",
	},
	"HostPath Volumes": {
		"hostPathVolumes",
	},
	"Sysctls": {
		"sysctls",
	},

	// Metadata-level control
	"AppArmor": {
		"appArmorProfile",
	},

	// === Restricted
	// Container and pod-level controls
	"Privilege Escalation": {
		"allowPrivilegeEscalation",
	},
	"Running as Non-root": {
		"runAsNonRoot",
	},
	"Running as Non-root user": {
		"runAsUser",
	},

	// Pod-level controls
	"Volume Types": {
		"restrictedVolumes",
	},
}

var PSS_controls = map[string][]restrictedField{
	// Control name as key, same as ID field in CheckResult

	// === Baseline
	// Container-level controls
	"privileged": {
		{
			// type:
			// - container-level
			// - pod-container-level
			// - pod level
			path: "spec.containers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"hostPorts": {
		{
			path: "spec.containers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			path: "spec.initContainers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
		{
			path: "spec.ephemeralContainers[*].ports[*].hostPort",
			allowedValues: []interface{}{
				false,
				0,
			},
		},
	},
	"procMount": {
		{
			path: "spec.containers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				"Default",
			},
		},
	},
	"capabilities_baseline": {
		{
			path: "spec.containers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
				"CHOWN",
				"DAC_OVERRIDE",
				"FOWNER",
				"FSETID",
				"KILL",
				"MKNOD",
				"NET_BIND_SERVICE",
				"SETFCAP",
				"SETGID",
				"SETPCAP",
				"SETUID",
				"SYS_CHROOT",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
				"CHOWN",
				"DAC_OVERRIDE",
				"FOWNER",
				"FSETID",
				"KILL",
				"MKNOD",
				"NET_BIND_SERVICE",
				"SETFCAP",
				"SETGID",
				"SETPCAP",
				"SETUID",
				"SYS_CHROOT",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"AUDIT_WRITE",
				"CHOWN",
				"DAC_OVERRIDE",
				"FOWNER",
				"FSETID",
				"KILL",
				"MKNOD",
				"NET_BIND_SERVICE",
				"SETFCAP",
				"SETGID",
				"SETPCAP",
				"SETUID",
				"SYS_CHROOT",
			},
		},
	},

	// Container and pod-level controls
	"windowsHostProcess": {
		{
			path: "spec.securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.containers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"seLinuxOptions": {
		// type
		{
			path: "spec.securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seLinuxOptions.type",
			allowedValues: []interface{}{
				"",
				"container_t",
				"container_init_t",
				"container_kvm_t",
			},
		},

		// user
		{
			path: "spec.securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.ephemeralContainers[*].seLinuxOptions.user",
			allowedValues: []interface{}{
				"",
			},
		},

		// role
		{
			path: "spec.securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.containers[*].securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
		{
			path: "spec.ephemeralContainers[*].seLinuxOptions.role",
			allowedValues: []interface{}{
				"",
			},
		},
	},
	"seccompProfile_baseline": {
		{
			path: "spec.securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.containers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				nil,
				"RuntimeDefault",
				"Localhost",
			},
		},
	},
	"seccompProfile_restricted": {
		{
			path: "spec.securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.containers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
			allowedValues: []interface{}{
				"RuntimeDefault",
				"Localhost",
			},
		},
	},

	// Pod-level controls
	"sysctls": {
		{
			path: "spec.securityContext.sysctls[*].name",
			allowedValues: []interface{}{
				"kernel.shm_rmid_forced",
				"net.ipv4.ip_local_port_range",
				"net.ipv4.tcp_syncookies",
				"net.ipv4.ping_group_range",
				"net.ipv4.ip_unprivileged_port_start",
			},
		},
	},
	"hostPathVolumes": {
		{
			path: "spec.volumes[*].hostPath",
			allowedValues: []interface{}{
				nil,
			},
		},
	},
	"hostNamespaces": {
		{
			path: "spec.hostNetwork",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.hostPID",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.hostIPC",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},

	// metadata-level controls
	"appArmorProfile": {
		{
			path: "metadata.annotations",
			allowedValues: []interface{}{
				nil,
				"",
				"runtime/default",
				"localhost/*",
			},
		},
	},

	// === Restricted
	"restrictedVolumes": {
		{
			path: "spec.volumes[*]",
			allowedValues: []interface{}{
				"spec.volumes[*].configMap",
				"spec.volumes[*].downwardAPI",
				"spec.volumes[*].emptyDir",
				"spec.volumes[*].projected",
				"spec.volumes[*].secret",
				"spec.volumes[*].csi",
				"spec.volumes[*].persistentVolumeClaim",
				"spec.volumes[*].ephemeral",
			},
		},
	},
	"runAsNonRoot": {
		{
			path: "spec.containers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				false,
				nil,
			},
		},
	},
	"runAsUser": {
		{
			path: "spec.securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.containers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.runAsUser",
			allowedValues: []interface{}{
				"",
				nil,
			},
		},
	},
	"allowPrivilegeEscalation": {
		{
			path: "spec.containers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
			allowedValues: []interface{}{
				false,
			},
		},
	},
	"capabilities_restricted": {
		{
			path: "spec.containers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.drop",
			allowedValues: []interface{}{
				"ALL",
			},
		},
		{
			path: "spec.containers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			path: "spec.initContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.add",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
	},
}

func containsCheckResult(s []PSSCheckResult, element policy.CheckResult) bool {
	for _, a := range s {
		if a.CheckResult == element {
			return true
		}
	}
	return false
}

func containsContainer(containers interface{}, containerName string) bool {
	switch v := containers.(type) {
	case []interface{}:
		for _, container := range v {
			switch v := container.(type) {
			case corev1.Container:
				if v.Name == containerName {
					return true
				}
			case corev1.EphemeralContainer:
				if v.Name == containerName {
					return true
				}
			}
		}
	case []corev1.Container:
		for _, container := range v {
			if container.Name == containerName {
				return true
			}
		}
	case []corev1.EphemeralContainer:
		for _, container := range v {
			if container.Name == containerName {
				return true
			}
		}
	}
	return false
}

// Get containers matching images specified in Exclude values
func containersMatchingImages(exclude []*kyvernov1.PodSecurityStandard, modularContainers interface{}) []interface{} {
	var matchingContainers []interface{}

	switch v := modularContainers.(type) {
	case []corev1.Container:
		for _, container := range v {
			for _, excludeRule := range exclude {
				if utils.ContainsString(excludeRule.Images, container.Image) {
					// Add to matchingContainers if either it's empty or is unique
					if len(matchingContainers) == 0 {
						matchingContainers = append(matchingContainers, container)
					} else if !containsContainer(matchingContainers, container.Name) {
						matchingContainers = append(matchingContainers, container)
					}
				}
			}
		}
	case []corev1.EphemeralContainer:
		for _, container := range v {
			for _, excludeRule := range exclude {
				if utils.ContainsString(excludeRule.Images, container.Image) {
					// Add to matchingContainers if either it's empty or is unique
					if len(matchingContainers) == 0 {
						matchingContainers = append(matchingContainers, container)
					} else if !containsContainer(matchingContainers, container.Name) {
						matchingContainers = append(matchingContainers, container)
					}
				}
			}
		}
	}
	return matchingContainers
}

// Get copy of pod with containers (containers, initContainers, ephemeralContainers) matching the exclude.image
func getPodWithMatchingContainers(exclude []*kyvernov1.PodSecurityStandard, pod *corev1.Pod) (podCopy corev1.Pod) {
	podCopy = *pod
	podCopy.Spec.Containers = []corev1.Container{}
	podCopy.Spec.InitContainers = []corev1.Container{}
	podCopy.Spec.EphemeralContainers = []corev1.EphemeralContainer{}

	for _, container := range pod.Spec.Containers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if strings.Contains(excludeRule.RestrictedField, "spec.containers[*]") && utils.ContainsString(excludeRule.Images, container.Image) ||
				controlNameOnly && utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.Containers) == 0 || !containsContainer(podCopy.Spec.Containers, container.Name) {
					podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.InitContainers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if strings.Contains(excludeRule.RestrictedField, "spec.initContainers[*]") && utils.ContainsString(excludeRule.Images, container.Image) ||
				controlNameOnly && utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.InitContainers) == 0 || !containsContainer(podCopy.Spec.InitContainers, container.Name) {
					podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		for _, excludeRule := range exclude {
			// Ignore all restrictedFields when we only specify the `controlName` with no `restrictedField`
			controlNameOnly := excludeRule.RestrictedField == ""
			if strings.Contains(excludeRule.RestrictedField, "spec.ephemeralContainers[*]") && utils.ContainsString(excludeRule.Images, container.Image) ||
				controlNameOnly && utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.EphemeralContainers) == 0 || !containsContainer(podCopy.Spec.EphemeralContainers, container.Name) {
					podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, container)
				}
			}
		}
	}
	return podCopy
}

// Get containers NOT matching images specified in Exclude values
func getPodWithNotMatchingContainers(exclude []*kyvernov1.PodSecurityStandard, pod *corev1.Pod, podWithMatchingContainers *corev1.Pod) (podCopy corev1.Pod) {
	// Only copy containers because we have already evaluated the pod-level controls
	// e.g.: spec.securityContext.hostProcess
	podCopy.Spec.Containers = []corev1.Container{}
	podCopy.Spec.InitContainers = []corev1.Container{}
	podCopy.Spec.EphemeralContainers = []corev1.EphemeralContainer{}

	// Append containers that are not in podWithMatchingContainers already evaluated in EvaluatePod()
	for _, container := range pod.Spec.Containers {
		if !containsContainer(podWithMatchingContainers.Spec.Containers, container.Name) {
			podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if !containsContainer(podWithMatchingContainers.Spec.InitContainers, container.Name) {
			podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		if !containsContainer(podWithMatchingContainers.Spec.EphemeralContainers, container.Name) {
			podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, container)
		}
	}
	return podCopy
}

// Get restrictedFields from Check.ID
func getRestrictedFields(check policy.Check) []restrictedField {
	for _, control := range PSS_controls_to_check_id {
		for _, checkID := range control {
			if check.ID == checkID {
				return PSS_controls[checkID]
			}
		}
	}
	return nil
}

// Evaluate Pod's specified containers only and get PSSCheckResults
func EvaluatePSS(level *api.LevelVersion, pod *corev1.Pod) (results []PSSCheckResult) {
	checks := policy.DefaultChecks()

	for _, check := range checks {
		// Restricted ? Baseline + Restricted (cumulative)
		// Baseline ? Then ignore checks for Restricted
		if level.Level == api.LevelBaseline && check.Level != level.Level {
			continue
		}
		// check version
		for _, versionCheck := range check.Versions {
			checkResult := versionCheck.CheckPod(&pod.ObjectMeta, &pod.Spec)
			// Append only if the checkResult is not already in PSSCheckResults
			if !checkResult.Allowed {
				results = append(results, PSSCheckResult{
					ID:               check.ID,
					CheckResult:      checkResult,
					RestrictedFields: getRestrictedFields(check),
				})
			}
		}
	}
	return results
}

func checkResultMatchesExclude(check PSSCheckResult, exclude *kyvernov1.PodSecurityStandard) bool {
	for _, restrictedField := range check.RestrictedFields {
		if restrictedField.path == exclude.RestrictedField {
			return true
		}
	}
	return false
}

// When we specify the controlName only we want to exclude all restrictedFields for this control
// so we remove all PSSChecks related to this control
func removePSSChecks(pssChecks []PSSCheckResult, rule *kyvernov1.PodSecurity) []PSSCheckResult {
	// Keep in memory the number of checks that have been removed
	// to avoid panics when removing a new check.
	removedChecks := 0
	for checkIndex, check := range pssChecks {
		for _, exclude := range rule.Exclude {
			// Translate PSS control to check_id and remove it from PSSChecks if it's specified in exclude block
			for _, CheckID := range PSS_controls_to_check_id[exclude.ControlName] {
				if check.ID == CheckID && exclude.RestrictedField == "" && checkIndex <= len(pssChecks) {
					index := checkIndex - removedChecks
					pssChecks = append(pssChecks[:index], pssChecks[index+1:]...)
					removedChecks++
				}
			}
		}
	}
	return pssChecks
}

func forbiddenValuesExempted(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, exclude *kyvernov1.PodSecurityStandard, restrictedField string) (bool, error) {
	if err := enginectx.AddJSONObject(ctx, pod); err != nil {
		return false, errors.Wrap(err, "failed to add podSpec to engine context")
	}

	// spec.containers[*].securityContext.privileged
	// -> spec.containers[?name=="nginx"].securityContext.privileged
	value, err := ctx.Query(restrictedField)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given path %s", exclude.RestrictedField))
	}
	if !allowedValues(value, *exclude, PSS_controls[check.ID]) {
		return false, nil
	}
	return true, nil
}

func checkContainerLevelFields(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, exclude []*kyvernov1.PodSecurityStandard, restrictedField restrictedField) (bool, error) {
	if strings.Contains(restrictedField.path, "spec.containers[*]") {
		for _, container := range pod.Spec.Containers {
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				if !strings.Contains(exclude.RestrictedField, "spec.containers[*]") {
					continue
				}

				//	Get values of this container only.
				// spec.containers[*].securityContext.privileged -> spec.containers[?name=="nginx"].securityContext.privileged
				newRestrictedField := strings.Replace(restrictedField.path, "*", fmt.Sprintf(`?name=='%s'`, container.Name), 1)

				// No need to check if exclude.Images contains container.Image
				// Since we only have containers matching the exclude.images with getPodWithMatchingContainers()

				exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, newRestrictedField)
				if err != nil || !exempted {
					return false, nil
				}
				matchedOnce = true
			}
			// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
			if strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) && !matchedOnce {
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.initContainers[*]") {
		for _, container := range pod.Spec.InitContainers {
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				if !strings.Contains(exclude.RestrictedField, "spec.initContainers[*]") {
					continue
				}

				//	Get values of this container only.
				// spec.containers[*].securityContext.privileged -> spec.containers[?name=="nginx"].securityContext.privileged
				newRestrictedField := strings.Replace(restrictedField.path, "*", fmt.Sprintf(`?name=='%s'`, container.Name), 1)

				exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, newRestrictedField)
				if err != nil || !exempted {
					return false, nil
				}
				matchedOnce = true
			}
			// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
			if strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) && !matchedOnce {
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.ephemeralContainers[*]") {
		for _, container := range pod.Spec.EphemeralContainers {
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				if !strings.Contains(exclude.RestrictedField, "spec.ephemeralContainers[*]") {
					continue
				}

				//	Get values of this container only.
				// spec.containers[*].securityContext.privileged -> spec.containers[?name=="nginx"].securityContext.privileged
				newRestrictedField := strings.Replace(restrictedField.path, "*", fmt.Sprintf(`?name=='%s'`, container.Name), 1)

				exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, newRestrictedField)
				if err != nil || !exempted {
					return false, nil
				}
				matchedOnce = true
			}
			// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
			if strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) && !matchedOnce {
				return false, nil
			}
		}
	}
	return true, nil
}

func checkPodLevelFields(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, rule *kyvernov1.PodSecurity, restrictedField restrictedField) (bool, error) {
	matchedOnce := false
	for _, exclude := range rule.Exclude {
		// No exclude for this specific pod-level restrictedField
		if !strings.Contains(exclude.RestrictedField, restrictedField.path) {
			continue
		}

		exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, exclude.RestrictedField)
		if err != nil || !exempted {
			return false, nil
		}
		matchedOnce = true
	}
	if !matchedOnce {
		return false, nil
	}
	return true, nil
}

func containsContainerLevelControl(restrictedFields []restrictedField) bool {
	for _, restrictedField := range restrictedFields {
		if strings.Contains(restrictedField.path, "ontainers[*]") {
			return true
		}
	}
	return false
}

func ExemptProfile(checks []PSSCheckResult, rule *kyvernov1.PodSecurity, pod *corev1.Pod) (bool, error) {
	ctx := enginectx.NewContext()

	// 1. Iterate over check.RestrictedFields
	// 2. Check if it's a `container-level` or `pod-level` restrictedField
	// - `container-level`: container has a disallowed check (container name in check.ForbiddenDetail) && exempted by an exclude rule ? continue : pod creation is forbbiden
	// - `pod-level`: Exempted by an exclude rule ? good : pod creation is forbbiden
	for _, check := range checks {
		for _, restrictedField := range check.RestrictedFields {
			// Is a container-level restrictedField
			if strings.Contains(restrictedField.path, "ontainers[*]") {
				allowed, err := checkContainerLevelFields(ctx, pod, check, rule.Exclude, restrictedField)
				if err != nil {
					return false, errors.Wrap(err, err.Error())
				}
				if !allowed {
					return false, nil
				}
			} else {
				// Is a pod-level restrictedField
				if !strings.Contains(check.CheckResult.ForbiddenDetail, "pod") && containsContainerLevelControl(check.RestrictedFields) {
					continue
				}
				allowed, err := checkPodLevelFields(ctx, pod, check, rule, restrictedField)
				if err != nil {
					return false, errors.Wrap(err, err.Error())
				}
				if !allowed {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

// Check if the pod creation is allowed after exempting some PSS controls
func EvaluatePod(rule *kyvernov1.PodSecurity, pod *corev1.Pod, level *api.LevelVersion) (bool, []PSSCheckResult, error) {
	// var podWithMatchingContainers corev1.Pod

	// 1. Evaluate containers that match images specified in exclude
	podWithMatchingContainers := getPodWithMatchingContainers(rule.Exclude, pod)

	pssChecks := EvaluatePSS(level, &podWithMatchingContainers)

	pssChecks = removePSSChecks(pssChecks, rule)

	// 2. Check if all PSSCheckResults are exempted by exclude values
	// Yes ? Evaluate pod's other containers
	// No ? Pod creation forbidden
	allowed, err := ExemptProfile(pssChecks, rule, &podWithMatchingContainers)
	if err != nil {
		return false, pssChecks, err
	}
	// Good to have: remove checks that are exempted and return only forbidden ones
	if !allowed {
		return false, pssChecks, nil
	}

	// 3. Optional, only when ExemptProfile() returns true
	podWithNotMatchingContainers := getPodWithNotMatchingContainers(rule.Exclude, pod, &podWithMatchingContainers)
	pssChecks = EvaluatePSS(level, &podWithNotMatchingContainers)
	if len(pssChecks) > 0 {
		return false, pssChecks, nil
	}
	return true, pssChecks, nil
}

func allowedValues(resourceValue interface{}, exclude kyvernov1.PodSecurityStandard, controls []restrictedField) bool {
	for _, control := range controls {
		if control.path == exclude.RestrictedField {
			for _, allowedValue := range control.allowedValues {
				switch v := allowedValue.(type) {
				case string:
					if !utils.ContainsString(exclude.Values, v) {
						exclude.Values = append(exclude.Values, v)
					}
					// case for nil pointers
				}
			}
		}
	}

	v := reflect.TypeOf(resourceValue)
	switch v.Kind() {
	case reflect.Bool:
		if !utils.ContainsString(exclude.Values, strconv.FormatBool(resourceValue.(bool))) {
			return false
		}
		return true
	case reflect.String:
		if !utils.ContainsString(exclude.Values, resourceValue.(string)) {
			return false
		}
		return true
	case reflect.Float64:
		if !utils.ContainsString(exclude.Values, fmt.Sprintf("%.f", resourceValue)) {
			return false
		}
		return true
	case reflect.Map:
		// `AppArmor` control
		for key, value := range resourceValue.(map[string]interface{}) {
			if !strings.Contains(key, "container.apparmor.security.beta.kubernetes.io/") {
				continue
			}
			// For allowed value: "localhost/*"
			if strings.Contains(value.(string), "localhost/") {
				continue
			}
			if !utils.ContainsString(exclude.Values, value.(string)) {
				return false
			}
		}
		return true
	}

	// Is an array
	excludeValues := resourceValue.([]interface{})

	for _, values := range excludeValues {
		v := reflect.TypeOf(values)
		switch v.Kind() {
		case reflect.Slice:
			for _, value := range values.([]interface{}) {
				if reflect.TypeOf(value).Kind() == reflect.Float64 {
					if !utils.ContainsString(exclude.Values, fmt.Sprintf("%.f", value)) {
						return false
					}
				} else if reflect.TypeOf(value).Kind() == reflect.String {
					if !utils.ContainsString(exclude.Values, value.(string)) {
						return false
					}
				}
			}
		case reflect.Map:
			for key, value := range values.(map[string]interface{}) {
				if exclude.RestrictedField == "spec.volumes[*]" {
					if key == "name" {
						continue
					}
					matchedOnce := false
					for _, excludeValue := range exclude.Values {
						// Remove `spec.volumes[*].` prefix
						if strings.TrimPrefix(excludeValue, "spec.volumes[*].") == key {
							matchedOnce = true
						}
					}
					if !matchedOnce {
						return false
					}
				}
				// "HostPath volume" control: check the path of the hostPath volume since the type is optional
				// volumes:
				// - name: test-volume
				//   hostPath:
				// 		# directory location on host
				// 		path: /data <--- Check the path
				// 		# this field is optional
				// 		type: Directory
				if exclude.RestrictedField == "spec.volumes[*].hostPath" {
					if key != "path" {
						continue
					}
					if !utils.ContainsString(exclude.Values, value.(string)) {
						return false
					}
				}

			}
		case reflect.String:
			if !utils.ContainsString(exclude.Values, values.(string)) {
				return false
			}

		case reflect.Bool:
			if !utils.ContainsString(exclude.Values, strconv.FormatBool(values.(bool))) {
				return false
			}
		case reflect.Float64:
			if !utils.ContainsString(exclude.Values, fmt.Sprintf("%.f", values)) {
				return false
			}
		}
	}
	return true
}
