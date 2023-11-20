package pss

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

// Evaluate Pod's specified containers only and get PSSCheckResults
func evaluatePSS(level *api.LevelVersion, pod corev1.Pod) (results []pssutils.PSSCheckResult) {
	checks := policy.DefaultChecks()
	var latestVersionCheck policy.VersionedCheck
	for _, check := range checks {
		if level.Level == api.LevelBaseline && check.Level != level.Level {
			continue
		}

		latestVersionCheck = check.Versions[0]
		for i := 1; i < len(check.Versions); i++ {
			vc := check.Versions[i]
			if !vc.MinimumVersion.Older(latestVersionCheck.MinimumVersion) {
				latestVersionCheck = vc
			}
		}

		if level.Version == api.LatestVersion() {
			checkResult := latestVersionCheck.CheckPod(&pod.ObjectMeta, &pod.Spec)
			if !checkResult.Allowed {
				results = append(results, pssutils.PSSCheckResult{
					ID:               string(check.ID),
					CheckResult:      checkResult,
					RestrictedFields: GetRestrictedFields(check),
				})
			}
		}

		for _, versionCheck := range check.Versions {
			// the latest check returned twice, skip duplicate application
			if level.Version == api.LatestVersion() {
				continue
			} else if level.Version != api.LatestVersion() && level.Version.Older(versionCheck.MinimumVersion) {
				continue
			}
			checkResult := versionCheck.CheckPod(&pod.ObjectMeta, &pod.Spec)
			// Append only if the checkResult is not already in pssCheckResult
			if !checkResult.Allowed {
				results = append(results, pssutils.PSSCheckResult{
					ID:               string(check.ID),
					CheckResult:      checkResult,
					RestrictedFields: GetRestrictedFields(check),
				})
			}
		}
	}
	return results
}

func exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults []pssutils.PSSCheckResult, exclude kyvernov1.PodSecurityStandard) []pssutils.PSSCheckResult {
	defaultCheckResultsMap := make(map[string]pssutils.PSSCheckResult, len(defaultCheckResults))

	for _, result := range defaultCheckResults {
		defaultCheckResultsMap[result.ID] = result
	}

	for _, excludeResult := range excludeCheckResults {
		for _, checkID := range pssutils.PSS_controls_to_check_id[exclude.ControlName] {
			if excludeResult.ID == checkID {
				delete(defaultCheckResultsMap, checkID)
			}
		}
	}

	var newDefaultCheckResults []pssutils.PSSCheckResult
	for _, result := range defaultCheckResultsMap {
		newDefaultCheckResults = append(newDefaultCheckResults, result)
	}

	return newDefaultCheckResults
}

func ParseVersion(level api.Level, version string) (*api.LevelVersion, error) {
	// Get pod security admission version
	var apiVersion api.Version

	// Version set to "latest" by default
	if version == "" || version == "latest" {
		apiVersion = api.LatestVersion()
	} else {
		parsedApiVersion, err := api.ParseVersion(version)
		if err != nil {
			return nil, err
		}
		apiVersion = api.MajorMinorVersion(parsedApiVersion.Major(), parsedApiVersion.Minor())
	}
	return &api.LevelVersion{
		Level:   level,
		Version: apiVersion,
	}, nil
}

// EvaluatePod applies PSS checks to the pod and exempts controls specified in the rule
func EvaluatePod(levelVersion *api.LevelVersion, excludes []kyvernov1.PodSecurityStandard, pod *corev1.Pod) (bool, []pssutils.PSSCheckResult) {
	// apply the pod security checks on pods
	defaultCheckResults := evaluatePSS(levelVersion, *pod)
	// exclude pod security controls if specified
	if len(excludes) > 0 {
		defaultCheckResults = ApplyPodSecurityExclusion(levelVersion, excludes, defaultCheckResults, pod)
	}

	return len(defaultCheckResults) == 0, defaultCheckResults
}

// ApplyPodSecurityExclusion excludes pod security controls
func ApplyPodSecurityExclusion(
	levelVersion *api.LevelVersion,
	excludes []kyvernov1.PodSecurityStandard,
	defaultCheckResults []pssutils.PSSCheckResult,
	pod *corev1.Pod,
) []pssutils.PSSCheckResult {
	for _, exclude := range excludes {
		spec, matching := GetPodWithMatchingContainers(exclude, pod)

		switch {
		// exclude pod level checks
		case spec != nil:
			excludeCheckResults := evaluatePSS(levelVersion, *spec)
			defaultCheckResults = exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults, exclude)

		// exclude container level checks
		default:
			excludeCheckResults := evaluatePSS(levelVersion, *matching)
			defaultCheckResults = exemptKyvernoExclusion(defaultCheckResults, excludeCheckResults, exclude)
		}
	}
	return defaultCheckResults
}

// GetPodWithMatchingContainers extracts matching container/pod info by the given exclude rule
// and returns pod manifests containing spec and container info respectively
func GetPodWithMatchingContainers(exclude kyvernov1.PodSecurityStandard, pod *corev1.Pod) (podSpec, matching *corev1.Pod) {
	if len(exclude.Images) == 0 {
		podSpec = pod.DeepCopy()
		podSpec.Spec.Containers = []corev1.Container{{Name: "fake"}}
		podSpec.Spec.InitContainers = nil
		podSpec.Spec.EphemeralContainers = nil
		return podSpec, nil
	}

	matchingImages := exclude.Images
	matching = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
		},
	}
	for _, container := range pod.Spec.Containers {
		if wildcard.CheckPatterns(matchingImages, container.Image) {
			matching.Spec.Containers = append(matching.Spec.Containers, container)
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if wildcard.CheckPatterns(matchingImages, container.Image) {
			matching.Spec.InitContainers = append(matching.Spec.InitContainers, container)
		}
	}

	for _, container := range pod.Spec.EphemeralContainers {
		if wildcard.CheckPatterns(matchingImages, container.Image) {
			matching.Spec.EphemeralContainers = append(matching.Spec.EphemeralContainers, container)
		}
	}

	return nil, matching
}

// Get restrictedFields from Check.ID
func GetRestrictedFields(check policy.Check) []pssutils.RestrictedField {
	for _, control := range pssutils.PSS_controls_to_check_id {
		for _, checkID := range control {
			if string(check.ID) == checkID {
				return pssutils.PSS_controls[checkID]
			}
		}
	}
	return nil
}

func FormatChecksPrint(checks []pssutils.PSSCheckResult) string {
	var str string
	for _, check := range checks {
		str += fmt.Sprintf("(%+v)\n", check.CheckResult)
	}
	return str
}
