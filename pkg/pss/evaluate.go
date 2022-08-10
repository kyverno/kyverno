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

	// Baseline
	"HostProcess": {
		"windowsHostProcess",
	},
	"Privileged Containers": {
		"privileged",
	},
	"Host Ports": {
		"hostPorts",
	},
	"SELinux": {
		"seLinuxOptions",
	},
	"/proc Mount Type": {
		"procMount",
	},
	"procMount": {
		"hostPorts",
	},

	// Restricted
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

	// Baseline
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
	"privileged": {
		{
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
				corev1.DefaultProcMount,
			},
		},
		{
			path: "spec.initContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				corev1.DefaultProcMount,
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.procMount",
			allowedValues: []interface{}{
				nil,
				corev1.DefaultProcMount,
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

	// Restricted
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
			path: "spec.initContainers[*].securityContext.capabilities.aad",
			allowedValues: []interface{}{
				nil,
				"NET_BIND_SERVICE",
			},
		},
		{
			path: "spec.ephemeralContainers[*].securityContext.capabilities.aad",
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
			if utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.Containers) == 0 || !containsContainer(podCopy.Spec.Containers, container.Name) {
					podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.InitContainers {
		for _, excludeRule := range exclude {
			if utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.InitContainers) == 0 || !containsContainer(podCopy.Spec.InitContainers, container.Name) {
					podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		for _, excludeRule := range exclude {
			if utils.ContainsString(excludeRule.Images, container.Image) {
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
func getPodWithNotMatchingContainers(exclude []*v1.PodSecurityStandard, pod *corev1.Pod) (podCopy corev1.Pod) {
	podCopy = *pod
	podCopy.Spec.Containers = []corev1.Container{}
	podCopy.Spec.InitContainers = []corev1.Container{}
	podCopy.Spec.EphemeralContainers = []corev1.EphemeralContainer{}

	// Set these restrictedFields to nil because we've tested them before in `ExemptProfile()`
	// HostProcess
	// podCopy.Spec.SecurityContext.WindowsOptions.HostProcess = nil

	for _, container := range pod.Spec.Containers {
		for _, excludeRule := range exclude {
			if !utils.ContainsString(excludeRule.Images, container.Image) && strings.Contains(excludeRule.RestrictedField, "spec.containers[*]") {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.Containers) == 0 || !containsContainer(podCopy.Spec.Containers, container.Name) {
					podCopy.Spec.Containers = append(podCopy.Spec.Containers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.InitContainers {
		for _, excludeRule := range exclude {
			if !utils.ContainsString(excludeRule.Images, container.Image) && strings.Contains(excludeRule.RestrictedField, "spec.initContainers[*]") {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.InitContainers) == 0 || !containsContainer(podCopy.Spec.InitContainers, container.Name) {
					podCopy.Spec.InitContainers = append(podCopy.Spec.InitContainers, container)
				}
			}
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		for _, excludeRule := range exclude {
			if !utils.ContainsString(excludeRule.Images, container.Image) && strings.Contains(excludeRule.RestrictedField, "spec.ephemeralContainers[*]") {
				// Add to matchingContainers if either it's empty or is unique
				if len(podCopy.Spec.EphemeralContainers) == 0 || !containsContainer(podCopy.Spec.EphemeralContainers, container.Name) {
					podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, container)
				}
			}
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

func ExemptProfile(checks []PSSCheckResult, rule *v1.PodSecurity, pod *corev1.Pod) (bool, error) {
	ctx := enginectx.NewContext()

	// Verify if every container present in the Check.FordibbenDetail is exempted
	// --> works only for controls with restrictedFields: containers, initContainers, ephemeralContainers
	// What about other pod-level restrictedFields ? spec.hostNetwork, spec.securityContext.windowsOptions.hostProcess etc ...
	for _, check := range checks {
		for _, container := range pod.Spec.Containers {
			fmt.Printf("\n[Container]: %+v\n", container)
			matchedOnce := false
			for _, exclude := range rule.Exclude {
				fmt.Printf("[Exclude]: %+v\n", exclude)
				// We can have multiple images in exclude block.
				for _, image := range exclude.Images {
					// Check only containers that are in PSSCheck.ForbiddenDetail and match the image in exclude.images
					if !strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) || !strings.Contains(exclude.RestrictedField, "spec.containers[*]") || container.Image != image {
						continue
					}
					if err := ctx.AddJSONObject(pod); err != nil {
						return false, errors.Wrap(err, "failed to add podSpec to engine context")
					}

					// spec.containers[?name=='nodejs'].securityContext.procMount
					value, err := ctx.Query(exclude.RestrictedField)
					fmt.Printf("==== image: %s\n", image)
					fmt.Printf("==== value: %s\n", value)
					if err != nil {
						return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
					}

					// // If exclude.Values is empty it means that we want to exclude all values for the restrictedField
					// if len(exclude.Values) == 0 {
					// 	return true, nil
					// }

					if !allowedValues(value, exclude) {
						return false, nil
					}
					matchedOnce = true
				}
			}
			if !matchedOnce {
				fmt.Printf("Container `%s` didn't match any exclude rule (container name must be in CheckResult.ForbiddenDetails and restrictedField match the container type)\n", container.Name)
				return false, nil
			}
		}
		for _, container := range pod.Spec.InitContainers {
			fmt.Printf("\n[InitContainer]: %+v\n", container)
			matchedOnce := false
			for _, exclude := range rule.Exclude {
				fmt.Printf("[Exclude]: %+v\n", exclude)
				for _, image := range exclude.Images {
					if !strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) || !strings.Contains(exclude.RestrictedField, "spec.initContainers[*]") || container.Image != image {
						continue
					}
					if err := ctx.AddJSONObject(pod); err != nil {
						return false, errors.Wrap(err, "failed to add podSpec to engine context")
					}

					value, err := ctx.Query(exclude.RestrictedField)
					if err != nil {
						return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
					}
					fmt.Printf("==== image: %s\n", image)
					fmt.Printf("==== value: %+v\n", value)
					// // If exclude.Values is empty it means that we want to exclude all values for the restrictedField
					// if len(exclude.Values) == 0 {
					// 	return true, nil
					// }

					if !allowedValues(value, exclude) {
						return false, nil
					}
					matchedOnce = true
				}
			}
			if !matchedOnce {
				fmt.Printf("Container `%s` didn't match any exclude rule (container name must be in CheckResult.ForbiddenDetails and restrictedField match the container type)\n", container.Name)
				return false, nil
			}
		}
		for _, container := range pod.Spec.EphemeralContainers {
			fmt.Printf("\n[ephemeralContainer]: %+v\n", container)
			matchedOnce := false
			for _, exclude := range rule.Exclude {
				fmt.Printf("[Exclude]: %+v\n", exclude)
				for _, image := range exclude.Images {
					// Check only containers that are in PSSCheck.ForbiddenDetail
					if !strings.Contains(check.CheckResult.ForbiddenDetail, container.Name) || !strings.Contains(exclude.RestrictedField, "spec.ephemeralContainers[*]") || container.Image != image {
						continue
					}
					if err := ctx.AddJSONObject(pod); err != nil {
						return false, errors.Wrap(err, "failed to add podSpec to engine context")
					}

					value, err := ctx.Query(exclude.RestrictedField)
					if err != nil {
						return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
					}

					// // If exclude.Values is empty it means that we want to exclude all values for the restrictedField
					// if len(exclude.Values) == 0 {
					// 	return true, nil
					// }

					if !allowedValues(value, exclude) {
						return false, nil
					}
					matchedOnce = true
				}
			}
			if !matchedOnce {
				fmt.Printf("Container `%s` didn't match any exclude rule (container name must be in CheckResult.ForbiddenDetails and restrictedField match the container type)\n", container.Name)
				return false, nil
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
	fmt.Println("\n== [EvaluatePSS, for containers that maches images specified in exclude] ==")

	podWithMatchingContainers = getPodWithMatchingContainers(rule.Exclude, pod)
	fmt.Printf("== [podWithMatchingContainers]: %+v\n", podWithMatchingContainers)

	pssChecks := EvaluatePSS(level, &podWithMatchingContainers)
	fmt.Printf("[PSSCheckResult]: %+v\n", pssChecks)

	pssChecks = removePSSChecks(pssChecks, rule)

	// 2. Check if all PSSCheckResults are exempted by exclude values
	// Yes ? Evaluate pod's other containers
	// No ? Pod creation forbidden
	fmt.Println("\n== [ExemptProfile] ==")
	allowed, err := ExemptProfile(pssChecks, rule, &podWithMatchingContainers)
	if err != nil {
		return false, pssChecks, err
	}
	// Good to have: remove checks that are exempted and return only forbidden ones
	if !allowed {
		return false, pssChecks, nil
	}

	// 3. Optional, only when ExemptProfile() returns true
	fmt.Println("\n== [EvaluatePSS, all PSSCheckResults were exempted by Exclude values. Evaluate other containers] ==")
	var podWithNotMatchingContainers corev1.Pod

	podWithNotMatchingContainers = getPodWithNotMatchingContainers(rule.Exclude, pod)
	fmt.Printf("== [podWithNotMatchingContainers]: %+v\n", podWithNotMatchingContainers)

	pssChecks = EvaluatePSS(level, &podWithNotMatchingContainers)
	fmt.Printf("[PSSCheckResult]: %+v\n", pssChecks)
	if len(pssChecks) > 0 {
		return false, pssChecks, nil
	}
	return true, pssChecks, nil
}

// only matches the rules
func imagesMatched(podSpec *corev1.PodSpec, images []string) bool {
	for _, container := range podSpec.Containers {
		if utils.ContainsString(images, container.Image) {
			return true
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

// default setting of the encoding/json decoder when unmarshaling JSON numbers into interface{} values.
// -----------------------------------------
// JSON booleans: bool
// JSON numbers: float64
// JSON strings: string
// JSON arrays: []interface{}
// JSON objects: map[string]interface{}
// JSON null: nil
func allowedValues(resourceValue interface{}, exclude *v1.PodSecurityStandard) bool {
	// Use `switch` keyword in golang

	// Is a Bool / String / Float
	// When resourceValue is a bool (Host Namespaces control)
	if reflect.TypeOf(resourceValue).Kind() == reflect.Bool {
		fmt.Printf("[exclude values]: %v\n[restricted field values]: %v\n", exclude.Values, resourceValue)
		if !utils.ContainsString(exclude.Values, strconv.FormatBool(resourceValue.(bool))) {
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
