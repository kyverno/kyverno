package pss

import (
	"fmt"
	"reflect"
	"strconv"

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

var PSS_Controls = map[string][]restrictedField{
	// Control name as key, same as ID field in CheckResult
	"privileged": {
		{
			path: "spec.containers[*].securityContext.privileged",
			allowedValues: []interface{}{
				false,
				nil,
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
	},
	"allowPrivilegeEscalation": {
		{
			path: "spec.containers[*].securityContext.allowPrivilegeEscalation",
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

func containsContainer(s []corev1.Container, element corev1.Container) bool {
	for _, a := range s {
		// Check container name because it's unique within a Pod
		if a.Name == element.Name {
			return true
		}
	}
	return false
}

// Get containers matching images specified in Exclude values
func ContainersMatchingImages(exclude []*v1.PodSecurityStandard, containers []corev1.Container) []corev1.Container {
	var matchingContainers []corev1.Container

	for _, container := range containers {
		for _, excludeRule := range exclude {
			if utils.ContainsString(excludeRule.Images, container.Image) {
				// Add to matchingContainers if either it's empty or is unique
				if len(matchingContainers) == 0 {
					matchingContainers = append(matchingContainers, container)
				} else if !containsContainer(matchingContainers, container) {
					matchingContainers = append(matchingContainers, container)
				}
			}
		}
	}
	return matchingContainers
}

// Get containers NOT matching images specified in Exclude values
func ContainersNotMatchingImages(exclude []*v1.PodSecurityStandard, containers []corev1.Container, matchingContainers []corev1.Container) []corev1.Container {
	var notMatchingContainers []corev1.Container

	for _, container := range containers {
		if !containsContainer(matchingContainers, container) {
			notMatchingContainers = append(notMatchingContainers, container)
		}
	}
	return notMatchingContainers
}

// Evaluate Pod's specified containers only and get PSSCheckResults
func EvaluatePSS(containers []corev1.Container, level *api.LevelVersion, podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) (results []PSSCheckResult) {
	checks := policy.DefaultChecks()

	// Remove containers that don't match images
	copyPodSpec := *podSpec
	copyPodSpec.Containers = containers

	fmt.Printf("[Containers]: %+v\n", containers)

	for _, check := range checks {

		// Restricted ? Baseline + Restricted (cumulative)
		// Baseline ? Then ignore checks for Restricted
		if level.Level == api.LevelBaseline && check.Level != level.Level {
			continue
		}

		for _, container := range containers {
			// check version
			for _, versionCheck := range check.Versions {
				checkResult := versionCheck.CheckPod(podMetadata, &copyPodSpec)
				// Append only if the checkResult is not already in PSSCheckResults
				if !checkResult.Allowed && !containsCheckResult(results, checkResult) {
					fmt.Printf("[Container]: %+v\n", container)
					fmt.Printf("[Check Error]: %+v\n", checkResult)
					results = append(results, PSSCheckResult{
						ID:               check.ID,
						CheckResult:      checkResult,
						RestrictedFields: PSS_Controls[check.ID],
					})
				}

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
	fmt.Printf("=== Before === %+v\n", pssChecks)
	for checkIndex, check := range pssChecks {
		for _, exclude := range rule.Exclude {
			if check.ID == exclude.ControlName && exclude.RestrictedField == "" {
				pssChecks = append(pssChecks[:checkIndex], pssChecks[checkIndex+1:]...)
			}
		}
	}
	fmt.Printf("=== After === %+v\n", pssChecks)
	return pssChecks

}

// Check if all PSSCheckResults are exempted by Exclude values
func ExemptProfile(checks []PSSCheckResult, matchingContainers []corev1.Container, rule *v1.PodSecurity, pod *corev1.Pod) (bool, error) {
	ctx := enginectx.NewContext()

	for _, check := range checks {
		matchedExcludeControl := false
		for excludeIndex, exclude := range rule.Exclude {
			// Check if PSSCheckResult matched an excluded control by looking at the restrictedField
			if !checkResultMatchesExclude(check, exclude) {
				// Return false if PSSCheckResult didn't match any exclude value
				if excludeIndex == len(rule.Exclude)-1 && !matchedExcludeControl {
					fmt.Printf("[PSSCheckResult didn't match any exclude value] Check: %+v\n", check)
					return false, nil
				} else {
					continue
				}
			}
			matchedExcludeControl = true

			fmt.Printf("\n[Check]: %+v\n", check)

			for _, container := range matchingContainers {
				// We can have the same Exclude.RestrictedField for multiple containers, select the right one
				if exclude.Images[0] != container.Image {
					continue
				}

				fmt.Printf("[Exclude]: %+v\n", exclude)
				fmt.Printf("[Container]: %+v\n", container)

				// if podObjectMeta != nil {
				// 	if err := ctx.AddJSONObject(podObjectMeta); err != nil {
				// 		return false, errors.Wrap(err, "failed to add podObjectMeta to engine context")
				// 	}
				// }
				// if podSpec != nil {
				if err := ctx.AddJSONObject(pod); err != nil {
					return false, errors.Wrap(err, "failed to add podSpec to engine context")
				}

				value, err := ctx.Query(exclude.RestrictedField)
				if err != nil {
					return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
				}

				// If exclude.Values is empty it means that we want to exclude all values for the restrictedField
				if len(exclude.Values) == 0 {
					return true, nil
				}

				if !allowedValues(value, exclude) {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

// Check if the pod creation is allowed after exempting some PSS controls
func EvaluatePod(rule *v1.PodSecurity, pod *corev1.Pod, level *api.LevelVersion) (bool, []PSSCheckResult, error) {
	// 1. Evaluate containers that match images specified in exclude
	fmt.Println("\n== [EvaluatePSS, for containers that maches images specified in exclude] ==")
	matchingContainers := ContainersMatchingImages(rule.Exclude, pod.Spec.Containers)
	pssChecks := EvaluatePSS(matchingContainers, level, &pod.ObjectMeta, &pod.Spec)
	fmt.Printf("[PSSCheckResult]: %+v\n", pssChecks)

	pssChecks = removePSSChecks(pssChecks, rule)

	// 2. Check if all PSSCheckResults are exempted by exclude values
	// Yes ? Evaluate pod's other containers
	// No ? Pod creation forbidden
	fmt.Println("\n== [ExemptProfile] ==")
	allowed, err := ExemptProfile(pssChecks, matchingContainers, rule, pod)
	if err != nil {
		return false, pssChecks, err
	}
	// Good to have: remove checks that are exempted and return only forbidden ones
	if !allowed {
		return false, pssChecks, nil
	}

	// 3. Optional, only when ExemptProfile() returns true
	fmt.Println("\n== [EvaluatePSS, all PSSCheckResults were exempted by Exclude values. Evaluate other containers] ==")
	notMatchingContainers := ContainersNotMatchingImages(rule.Exclude, pod.Spec.Containers, matchingContainers)
	pssChecks = EvaluatePSS(notMatchingContainers, level, &pod.ObjectMeta, &pod.Spec)
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

// func EvaluatePSS(lv api.LevelVersion, podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) (results []PSSCheckResult) {
// 	checks := policy.DefaultChecks()

// 	for _, check := range checks {

// 		// Restricted ? Baseline + Restricted (cumulative)
// 		// Baseline ? Then ignore checks for Restricted
// 		// fmt.Printf("current level: %s, check level: %s\n", lv.Level, check.Level)
// 		if lv.Level == api.LevelBaseline && check.Level != lv.Level {
// 			continue
// 		}

// 		// check version
// 		for _, versionCheck := range check.Versions {
// 			res := versionCheck.CheckPod(podMetadata, podSpec)

// 			// when pod creation is forbidden
// 			// if the control name is in the exclude
// 			if !res.Allowed {
// 				fmt.Printf("[Check Error]: %+v\n", res)
// 				results = append(results, PSSCheckResult{
// 					ID:               check.ID,
// 					CheckResult:      res,
// 					RestrictedFields: PSS_Controls[check.ID],
// 				})
// 			}
// 		}
// 	}
// 	return results
// }

// If the returned error from EvaluatePSS is exempted
// func ExemptProfile(rule *v1.PodSecurity, podSpec *corev1.PodSpec, podObjectMeta *metav1.ObjectMeta) (bool, error) {
// 	ctx := enginectx.NewContext()

// 	for _, exclude := range rule.Exclude {
// 		for _, container := range podSpec.Containers {
// 			// Check if the container image matches the image specified in exclude
// 			if !utils.ContainsString(exclude.Images, container.Image) {
// 				continue
// 			}
// 			fmt.Printf("[Container]: %+v\n", container)
// 			// double check if the given RestrictedField violates the specific profile?

// 			// need a RestrictedField - check ID map to fetch psa Check

// 			// if podObjectMeta != nil {
// 			// 	if err := ctx.AddJSONObject(podObjectMeta); err != nil {
// 			// 		return false, errors.Wrap(err, "failed to add podObjectMeta to engine context")
// 			// 	}
// 			// }
// 			// if podSpec != nil {
// 			if err := ctx.AddJSONObject(container); err != nil {
// 				return false, errors.Wrap(err, "failed to add podSpec to engine context")
// 			}
// 			// }

// 			value, err := ctx.Query(exclude.RestrictedField)
// 			if err != nil {
// 				return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
// 			}

// 			fmt.Printf("[Exclude]: %+v\n", exclude)

// 			// If exclude.Values is empty it means that we want to exclude all values for the restrictedField
// 			if len(exclude.Values) == 0 {
// 				return true, nil
// 			}

// 			if !allowedValues(value, exclude) {
// 				return false, nil
// 			}
// 		}
// 	}
// 	return true, nil
// }

// Create issue, PR
// {
// 	"Allowed":false,
// 	"ForbiddenReason":"non-default capabilities",
// 	"ForbiddenDetail":"containers "init-container-nginx-host-network", "nginx-host-network" must not include "SYS_ADMIN", "SYS_NICE" in securityContext.capabilities.add",

// 	// Fields to add
// 	"RestrictedFields":[
// 	{
// 		"Path":"spec.containers[*].securityContext.capabilities.add",
// 		"ForbiddenValues": {
// 			"nginx-host-network": [
// 				"SYS_ADMIN",
// 				"SYS_NICE"
// 			],
// 			"another_nginx": [
// 				"SYS_ADMIN",
// 				"SYS_NICE",
// 				"SYS_TIME"
// 			],
// 		}
// 	 }
// 	]
// }
