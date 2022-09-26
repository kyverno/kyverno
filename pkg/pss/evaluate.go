package pss

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

// Evaluate Pod's specified containers only and get PSSCheckResults
func evaluatePSS(level *api.LevelVersion, pod *corev1.Pod) (results []pssCheckResult) {
	checks := policy.DefaultChecks()

	for _, check := range checks {
		if level.Level == api.LevelBaseline && check.Level != level.Level {
			continue
		}
		// check version
		for _, versionCheck := range check.Versions {
			checkResult := versionCheck.CheckPod(&pod.ObjectMeta, &pod.Spec)
			// Append only if the checkResult is not already in pssCheckResult
			if !checkResult.Allowed {
				results = append(results, pssCheckResult{
					id:               check.ID,
					checkResult:      checkResult,
					restrictedFields: getRestrictedFields(check),
				})
			}
		}
	}
	return results
}

func exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults []pssCheckResult, exclude kyvernov1.PodSecurityStandard) []pssCheckResult {
	defaultCheckResultsMap := make(map[string]pssCheckResult, len(defaultCheckResults))

	for _, result := range defaultCheckResults {
		defaultCheckResultsMap[result.id] = result
	}

	for _, excludeResult := range excludeCheckResults {
		for _, checkID := range PSS_controls_to_check_id[exclude.ControlName] {
			if excludeResult.id == checkID {
				delete(defaultCheckResultsMap, checkID)
			}
		}
	}

	var newDefaultCheckResults []pssCheckResult
	for _, result := range defaultCheckResultsMap {
		newDefaultCheckResults = append(newDefaultCheckResults, result)
	}

	return newDefaultCheckResults
}

// Check if the pod creation is allowed after exempting some PSS controls
func EvaluatePod(rule *kyvernov1.PodSecurity, pod *corev1.Pod, level *api.LevelVersion) (bool, []pssCheckResult, error) {
	// .exclude defines the excluded control name
	// 1. apply PSS to the entire pods, checkResults
	// 2. for loop extract matching containers for each exclude, at the pod or container level, and applies
	// 3. exempt control checks by kyverno rule
	//    - still violates, block, return false
	//    - allow, continue

	defaultCheckResults := evaluatePSS(level, pod)

	for _, exclude := range rule.Exclude {
		spec, matching := getPodWithMatchingContainers(exclude, pod)

		switch {
		// exclude pod level checks
		case spec != nil:
			excludeCheckResults := evaluatePSS(level, spec)
			defaultCheckResults = exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults, exclude)

		// exclude container level checks
		default:
			excludeCheckResults := evaluatePSS(level, matching)
			defaultCheckResults = exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults, exclude)
		}
	}

	return len(defaultCheckResults) == 0, defaultCheckResults, nil
}
