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

func FormatChecksPrint(checks []PSSCheckResult) string {
	var str string
	for _, check := range checks {
		str += fmt.Sprintf("(%+v)\n", check.CheckResult)
	}
	return str
}

// Evaluate Pod's specified containers only and get PSSCheckResults
func evaluatePSS(level *api.LevelVersion, pod *corev1.Pod) (results []PSSCheckResult) {
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

// When we specify the controlName only we want to exclude all restrictedFields for this control.
// Remove all PSSChecks related to this control
func trimExemptedChecks(pssChecks []PSSCheckResult, rule *kyvernov1.PodSecurity) []PSSCheckResult {
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

func forbiddenValuesExempted(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, exclude kyvernov1.PodSecurityStandard, restrictedField string) (bool, error) {
	if err := enginectx.AddJSONObject(ctx, pod); err != nil {
		return false, errors.Wrap(err, "failed to add podSpec to engine context")
	}
	value, err := ctx.Query(restrictedField)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given path %s", exclude.RestrictedField))
	}
	if !allowedValues(value, exclude, PSS_controls[check.ID]) {
		return false, nil
	}
	return true, nil
}

func checkContainer(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, exclude []kyvernov1.PodSecurityStandard, restrictedField restrictedField, containerName string, containerTypePrefix string) (bool, error) {
	matchedOnce := false
	// Container.Name with double quotes
	formatedContainerName := fmt.Sprintf(`"%s"`, containerName)
	if !strings.Contains(check.CheckResult.ForbiddenDetail, formatedContainerName) {
		return true, nil
	}
	for _, exclude := range exclude {
		if !strings.Contains(exclude.RestrictedField, containerTypePrefix) {
			continue
		}

		// Get values of this container only.
		// spec.containers[*].securityContext.privileged -> spec.containers[?name=="nginx"].securityContext.privileged
		newRestrictedField := strings.Replace(restrictedField.path, "*", fmt.Sprintf(`?name=='%s'`, containerName), 1)

		// No need to check if exclude.Images contains container.Image
		// Since we only have containers matching the exclude.images with getPodWithMatchingContainers()
		exempted, err := forbiddenValuesExempted(ctx, pod, check, exclude, newRestrictedField)
		if err != nil || !exempted {
			return false, nil
		}
		matchedOnce = true
	}
	// If container name is in check.Forbidden but isn't exempted by an exclude then pod creation is forbidden
	if strings.Contains(check.CheckResult.ForbiddenDetail, formatedContainerName) && !matchedOnce {
		return false, nil
	}
	return true, nil
}

func checkContainerLevelFields(ctx enginectx.Interface, pod *corev1.Pod, check PSSCheckResult, exclude []kyvernov1.PodSecurityStandard, restrictedField restrictedField) (bool, error) {
	if strings.Contains(restrictedField.path, "spec.containers[*]") {
		for _, container := range pod.Spec.Containers {
			allowed, err := checkContainer(ctx, pod, check, exclude, restrictedField, container.Name, "spec.containers[*]")
			if err != nil || !allowed {
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.initContainers[*]") {
		for _, container := range pod.Spec.InitContainers {
			allowed, err := checkContainer(ctx, pod, check, exclude, restrictedField, container.Name, "spec.initContainers[*]")
			if err != nil || !allowed {
				return false, nil
			}
		}
	}
	if strings.Contains(restrictedField.path, "spec.ephemeralContainers[*]") {
		for _, container := range pod.Spec.EphemeralContainers {
			allowed, err := checkContainer(ctx, pod, check, exclude, restrictedField, container.Name, "spec.ephemeralContainers[*]")
			if err != nil || !allowed {
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
	// 1. Evaluate containers that match images specified in exclude
	podWithMatchingContainers := getPodWithMatchingContainers(rule.Exclude, pod)
	pssChecks := evaluatePSS(level, &podWithMatchingContainers)
	pssChecks = trimExemptedChecks(pssChecks, rule)

	// 2. Check if all PSSCheckResults are exempted by exclude values
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
	pssChecks = evaluatePSS(level, &podWithNotMatchingContainers)
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
