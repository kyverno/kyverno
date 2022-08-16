package pss

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

// Problems to address
// 1. JMESPath:
// - in PodSpec by default (don't need to specify "spec." prefix in RestrictedField)
// - Problem with App Armor: cannot query Metadata field inside Pod

// --> Solution: Add Pod object we send `ctx.AddJSONObject(podSpec)`

// 2. HostPathVolumes: container has an allowed volumeSource (emptyDir) —> ExemptProfile() fails
// exclude.Values = ["hostPath"]
// key = hostPath (not allowed), emptyDir (allowed)
// if !utils.ContainsString(exclude.Values, key) {
//     return false
// }

// --> Solution:

// Add specific conditions / PSS control:
// Check the restrictedField and concat allowedValues to excludeValues
// - if conditions (https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_hostPorts.go)
// - array of allowed values (https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_seLinuxOptions.go)

// --> Solution: skip allowed values

// 3. Values []string -> []interface{} ? HostPorts
// Have to check if the object inside []interface{} is a:

// - String
// - Float64
// - Bool
// ….

// --> Solution: Use switch to make it more readable

// 4. Cannot find Running as Non-Root User control files in K8S repo

// --> Solution: https://github.com/kubernetes/pod-security-admission/blob/master/policy/check_runAsUser_test.go

// 5. ExcludeValue: undefined for restricted seccomp

// --> Solution: either undefined / null

// TO DO:
// E2E test for one control
// 1. New package for PSS checks, connect with Kyverno Engine (admission webhook)

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

	// === Restricted
	"Running as Non-root": {
		{
			path: "spec.securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
			},
		},
		{
			path: "spec.containers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
			allowedValues: []interface{}{
				true,
			},
		},
	},
	"Running as Non-root user": {
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
			fmt.Printf("container name: %s\n", container.Name)
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
func containersMatchingImages(exclude []*v1.PodSecurityStandard, modularContainers interface{}) []interface{} {
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
func getPodWithMatchingContainers(exclude []*v1.PodSecurityStandard, pod *corev1.Pod) (podCopy corev1.Pod) {
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
func getPodWithNotMatchingContainers(exclude []*v1.PodSecurityStandard, pod *corev1.Pod, podWithMatchingContainers *corev1.Pod) (podCopy corev1.Pod) {
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
				// fmt.Printf("[Container]: %+v\n", container)
				fmt.Printf("[Check Error]: %+v\n", checkResult)
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

func checkResultMatchesExclude(check PSSCheckResult, exclude *v1.PodSecurityStandard) bool {
	for _, restrictedField := range check.RestrictedFields {
		if restrictedField.path == exclude.RestrictedField {
			return true
		}
	}
	return false
}

// When we specify the controlName only we want to exclude all restrictedFields for this control
// so we remove all PSSChecks related to this control
func removePSSChecks(pssChecks []PSSCheckResult, rule *v1.PodSecurity) []PSSCheckResult {
	fmt.Printf("=== Remove all restrictedFields when we only specify the controlName.\n")
	fmt.Printf("=== Before: %+v\n", pssChecks)

	// Keep in memory the number of checks that have been removed
	// to avoid panics when removing a new check.
	removedChecks := 0
	for checkIndex, check := range pssChecks {
		fmt.Printf("======= Check: %+v\n", check)
		for _, exclude := range rule.Exclude {
			// Translate PSS control to check_id and remove it from PSSChecks if it's specified in exclude block
			for _, CheckID := range PSS_controls_to_check_id[exclude.ControlName] {
				if check.ID == CheckID && exclude.RestrictedField == "" && checkIndex <= len(pssChecks) {
					fmt.Printf("=== check.ID to remove: %s\n", check.ID)
					index := checkIndex - removedChecks
					pssChecks = append(pssChecks[:index], pssChecks[index+1:]...)
					removedChecks++
				}
			}
		}
	}
	fmt.Printf("=== After: %+v\n", pssChecks)
	return pssChecks

}

func forbiddenValuesExempted(ctx *enginectx.Context, pod *corev1.Pod, check PSSCheckResult, exclude *v1.PodSecurityStandard, restrictedField string) (bool, error) {
	if err := ctx.AddJSONObject(pod); err != nil {
		return false, errors.Wrap(err, "failed to add podSpec to engine context")
	}

	// spec.containers[*].securityContext.privileged
	// -> spec.containers[?name=="nginx"].securityContext.privileged
	value, err := ctx.Query(restrictedField)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given path %s", exclude.RestrictedField))
	}
	fmt.Printf("=== Value: %+v\n", value)
	if !allowedValues(value, *exclude, PSS_controls[check.ID]) {
		return false, nil
	}
	return true, nil
}

func checkContainerLevelFields(ctx *enginectx.Context, pod *corev1.Pod, check PSSCheckResult, exclude []*v1.PodSecurityStandard, restrictedField *restrictedField) (bool, error) {
	fmt.Printf("=== Is a container-level restrictedField\n")

	if strings.Contains(restrictedField.path, "spec.containers[*]") {
		for _, container := range pod.Spec.Containers {
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			fmt.Printf("ContainerName: %s\n", containerName)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				fmt.Printf("=== exclude.RestrictedField: %s\n", exclude.RestrictedField)
				if !strings.Contains(exclude.RestrictedField, "spec.containers[*]") {
					fmt.Println("2")
					continue
				}

				fmt.Printf("=== Container: `%+v`\n", container)

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
				fmt.Printf("====== MATCHED\n")
			}
			// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
			if strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) && !matchedOnce {
				fmt.Printf("=== Container `%s` didn't match any exclude rule.\n", container.Name)
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.initContainers[*]") {
		for _, container := range pod.Spec.InitContainers {
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			fmt.Printf("ContainerName: %s\n", containerName)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				fmt.Printf("=== exclude.RestrictedField: %s\n", exclude.RestrictedField)
				if !strings.Contains(exclude.RestrictedField, "spec.initContainers[*]") {
					continue
				}
				fmt.Printf("=== initContainer: `%+v`\n", container)

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
				fmt.Printf("=== initContainer `%s` didn't match any exclude rule.\n", container.Name)
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.ephemeralContainers[*]") {
		for _, container := range pod.Spec.EphemeralContainers {
			fmt.Printf("=== ephemeralContainer: `%+v`\n", container)
			matchedOnce := false
			// Container.Name with double quotes
			containerName := fmt.Sprintf(`"%s"`, container.Name)
			fmt.Printf("ContainerName: %s\n", containerName)
			if !strings.Contains(check.CheckResult.ForbiddenDetail, containerName) {
				continue
			}
			for _, exclude := range exclude {
				fmt.Printf("=== exclude.RestrictedField: %s\n", exclude.RestrictedField)
				if !strings.Contains(exclude.RestrictedField, "spec.ephemeralContainers[*]") {
					continue
				}
				fmt.Printf("=== ephemeralContainer: `%+v`\n", container)

				//	Get values of this container only.
				// spec.containers[*].securityContext.privileged -> spec.containers[?name=="nginx"].securityContext.privileged
				newRestrictedField := strings.Replace(restrictedField.path, "*", fmt.Sprintf(`?name=='%s'`, container.Name), 1)

				if err := ctx.AddJSONObject(pod); err != nil {
					return false, errors.Wrap(err, "failed to add podSpec to engine context")
				}

				exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, newRestrictedField)
				if err != nil || !exempted {
					return false, nil
				}
				matchedOnce = true
			}
			// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
			if strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) && !matchedOnce {
				fmt.Printf("=== ephemeralContainer `%s` didn't match any exclude rule.\n", container.Name)
				return false, nil
			}
		}
	}
	fmt.Println("=== allowed")
	return true, nil
}

func checkPodLevelFields(ctx *enginectx.Context, pod *corev1.Pod, check PSSCheckResult, rule *v1.PodSecurity, restrictedField *restrictedField) (bool, error) {
	fmt.Printf("=== Is a pod-level restrictedField\n")

	// Unlike containers, we don't know which pod-level restrictedField is forbidden.

	// Check if value is allowed
	// podCopy := corev1.Pod{}
	//

	// if err := ctx.AddJSONObject(pod); err != nil {
	// 	return false, errors.Wrap(err, "failed to add podSpec to engine context")
	// }

	// value, err := ctx.Query(restrictedField.path)
	// // fmt.Printf("=== restrictedField.path: %+v\n", restrictedField.path)
	// // Return an error if the value is nil:
	// if err != nil {

	// 	return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given path %s", restrictedField.path))
	// }
	// fmt.Printf("=== Value: %+v\n", value)

	// if !allowedValues(value, exclude, PSS_controls[check.ID]) {
	// 	return false, nil
	// }

	// reflect.ValueOf(podCopy).Elem().FieldByName(restrictedField.path).Set(value)
	// fmt.Printf("-------- podCopy: %+v", podCopy)

	// 	EvaluatePSS(rule.Level)
	// yes -> next restrictedField
	// no -> check if an exclude exempt the forbidden value

	matchedOnce := false
	for _, exclude := range rule.Exclude {
		// No exclude for this specific pod-level restrictedField
		if !strings.Contains(exclude.RestrictedField, restrictedField.path) {
			continue
		}
		if err := ctx.AddJSONObject(pod); err != nil {
			return false, errors.Wrap(err, "failed to add podSpec to engine context")
		}

		exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, exclude.RestrictedField)
		if err != nil || !exempted {
			return false, nil
		}
		matchedOnce = true
	}
	if !matchedOnce {
		fmt.Println("=== Didn't match any exclude rule")
		return false, nil
	}
	fmt.Println("=== allowed")
	return true, nil
}

func ExemptProfile(checks []PSSCheckResult, rule *v1.PodSecurity, pod *corev1.Pod) (bool, error) {
	ctx := enginectx.NewContext()

	// 1. Iterate over check.RestrictedFields
	// 2. Check if it's a `container-level` or `pod-level` restrictedField
	// - `container-level`: container has a disallowed check (container name in check.ForbiddenDetail) && exempted by an exclude rule ? good : pod creation is forbbiden
	// - `pod-level`: Exempted by an exclude rule ? good : pod creation is forbbiden

	// Problems:
	// 1. When we have a control with multiple `pod-level` restrictedFields. How to check if a specific RestrictedField is disallowed by the check ?
	// e.g.: `Host Namespaces` control:

	// 2. `Container-level` restrictedField that can have multiple values (capabilities), we have to get the values for each container not for every containers:
	// `spec.containers[*].securityContext.capabilities.add` --> `spec.containers[?name==nginx].securityContext.capabilities.add`
	for _, check := range checks {
		fmt.Printf("\n===== Check: %+v\n", check)
		for _, restrictedField := range check.RestrictedFields {
			fmt.Printf("\n=== restrictedField: %s\n", restrictedField.path)
			// Is a container-level restrictedField
			if strings.Contains(restrictedField.path, "ontainers[*]") {
				allowed, err := checkContainerLevelFields(ctx, pod, check, rule.Exclude, &restrictedField)
				if err != nil {
					return false, errors.Wrap(err, err.Error())
				}
				if !allowed {
					return false, nil
				}
			}
			// Is a pod-level restrictedField
			if !strings.Contains(restrictedField.path, "ontainers[*]") {
				// if !strings.Contains(check.CheckResult.ForbiddenDetail, "pod") {
				// 	continue
				// }
				allowed, err := checkPodLevelFields(ctx, pod, check, rule, &restrictedField)
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
func EvaluatePod(rule *v1.PodSecurity, pod *corev1.Pod, level *api.LevelVersion) (bool, []PSSCheckResult, error) {
	// var matchingContainers []interface{}
	var podWithMatchingContainers corev1.Pod

	// 1. Evaluate containers that match images specified in exclude
	fmt.Println("\n=== [EvaluatePSS, for containers that matches images specified in exclude] ==")

	podWithMatchingContainers = getPodWithMatchingContainers(rule.Exclude, pod)
	fmt.Printf("=== [podWithMatchingContainers]: %+v\n", podWithMatchingContainers)

	pssChecks := EvaluatePSS(level, &podWithMatchingContainers)
	fmt.Printf("[PSSCheckResult]: %+v\n", pssChecks)

	pssChecks = removePSSChecks(pssChecks, rule)

	// 2. Check if all PSSCheckResults are exempted by exclude values
	// Yes ? Evaluate pod's other containers
	// No ? Pod creation forbidden
	fmt.Println("\n=== [ExemptProfile] ===")
	allowed, err := ExemptProfile(pssChecks, rule, &podWithMatchingContainers)
	if err != nil {
		return false, pssChecks, err
	}
	// Good to have: remove checks that are exempted and return only forbidden ones
	if !allowed {
		return false, pssChecks, nil
	}

	// 3. Optional, only when ExemptProfile() returns true
	fmt.Println("\n=== [EvaluatePSS, all PSSCheckResults were exempted by Exclude values. Evaluate other containers] ==")
	var podWithNotMatchingContainers corev1.Pod

	podWithNotMatchingContainers = getPodWithNotMatchingContainers(rule.Exclude, pod, &podWithMatchingContainers)
	fmt.Printf("=== [podWithNotMatchingContainers]: %+v\n", podWithNotMatchingContainers)

	pssChecks = EvaluatePSS(level, &podWithNotMatchingContainers)
	fmt.Printf("[PSSCheckResult]: %+v\n", pssChecks)
	if len(pssChecks) > 0 {
		return false, pssChecks, nil
	}
	return true, pssChecks, nil
}

// only matches the rules
func imagesMatched(containers interface{}, images []string) bool {
	switch v := containers.(type) {
	case []corev1.Container:
		for _, container := range v {
			if utils.ContainsString(images, container.Image) {
				return true
			}
		}
	case []corev1.EphemeralContainer:
		for _, container := range v {
			if utils.ContainsString(images, container.Image) {
				return true
			}
		}
	}
	return false
}

func namespaceMatched(podMetadata *metav1.ObjectMeta, namespace string) bool {
	fmt.Printf("podMetadata.Namespace: %s\n", podMetadata.Namespace)
	fmt.Printf("namespace: %s\n", namespace)
	if podMetadata.Namespace == namespace {
		return true
	}
	return false
}

func containsBool(list []bool, value bool) {

}

// default setting of the encoding/json decoder when unmarshaling JSON numbers into interface{} values.
// -----------------------------------------
// JSON booleans: bool
// JSON numbers: float64
// JSON strings: string
// JSON arrays: []interface{}
// JSON objects: map[string]interface{}
// JSON null: nil
func allowedValues(resourceValue interface{}, exclude v1.PodSecurityStandard, controls []restrictedField) bool {
	// Use `switch` keyword in golang
	fmt.Printf("====== Before exclude.Values: %+v\n", exclude.Values)

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
	fmt.Printf("====== After exclude.Values: %+v\n", exclude.Values)

	// Is a Bool / String / Float
	// When resourceValue is a bool (Host Namespaces control)
	if reflect.TypeOf(resourceValue).Kind() == reflect.Bool {
		fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, resourceValue)
		if !utils.ContainsString(exclude.Values, strconv.FormatBool(resourceValue.(bool))) {
			return false
		}
		return true
	}
	if reflect.TypeOf(resourceValue).Kind() == reflect.String {
		fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, resourceValue)
		if !utils.ContainsString(exclude.Values, resourceValue.(string)) {
			return false
		}
		return true
	}

	// Is an array
	excludeValues := resourceValue.([]interface{})

	// // Allow a RestrictedField to be undefined (Restricted Seccomp control)
	if len(exclude.Values) == 1 && exclude.Values[0] == "undefined" {
		if len(excludeValues) == 0 {
			return true
		}
		return false
	}

	for _, values := range excludeValues {
		rt := reflect.TypeOf(values)
		kind := rt.Kind()

		if kind == reflect.Slice {
			fmt.Println(values, "is a slice with element type", rt.Elem())
			for _, value := range values.([]interface{}) {
				fmt.Printf("value: %s\n", value)

				// Check value type
				fmt.Printf("type: %s\n", reflect.TypeOf(value).Kind())
				if reflect.TypeOf(value).Kind() == reflect.Float64 {
					fmt.Println(fmt.Sprint((value.(float64))))
					if !utils.ContainsString(exclude.Values, fmt.Sprint((value.(float64)))) {
						return false
					}
				} else if reflect.TypeOf(value).Kind() == reflect.String {
					fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, value)
					if !utils.ContainsString(exclude.Values, value.(string)) {
						return false
					}
				}
			}
		} else if kind == reflect.Map {
			// For Volume Types control
			fmt.Println(values, "is a map with element type", rt.Elem())
			for key, value := range values.(map[string]interface{}) {
				// `Volume`` has 2 fields: `Name` and a `Volume Source` (inline json)
				// Ignore `Name` field because we want to look at `Volume Source`'s key
				// https://github.com/kubernetes/api/blob/f18d381b8d0129e7098e1e67a89a8088f2dba7e6/core/v1/types.go#L36
				if key == "name" {
					continue
				}
				fmt.Printf("[exclude values]: %v\n[key]: %s\n[restricted field values]: %v\n", exclude.Values, key, value)
				if !utils.ContainsString(exclude.Values, key) {
					return false
				}
			}
		} else if kind == reflect.String {
			fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, values)
			if !utils.ContainsString(exclude.Values, values.(string)) {
				return false
			}

		} else if kind == reflect.Bool {
			fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, values)
			if !utils.ContainsString(exclude.Values, strconv.FormatBool(values.(bool))) {
				return false
			}
		} else {
			fmt.Println(values, "is something else entirely")
		}
		return true
	}
	return true
}
