package pss

import (
	"fmt"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/pod-security-admission/api"
	"k8s.io/pod-security-admission/policy"
)

func EvaluatePSS(lv api.LevelVersion, podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) (results []policy.CheckResult) {
	checks := policy.DefaultChecks()
	for _, check := range checks {
		if check.Level != lv.Level {
			continue
		}

		// check version

		for _, versionCheck := range check.Versions {
			res := versionCheck.CheckPod(podMetadata, podSpec)
			if !res.Allowed {
				fmt.Println(res)
				results = append(results, res)
			}
		}
	}

	return
}

func ExemptProfile(rule *v1.PodSecurity, podSpec *corev1.PodSpec) (bool, error) {
	ctx := enginectx.NewContext()

	for _, exclude := range rule.Exclude {
		if !imagesMatched(podSpec, exclude.Images) {
			continue
		}

		// double check if the given path violates the specific profile?
		// need a path - check ID map to fetch psa Check

		if err := ctx.AddJSONObject(podSpec); err != nil {
			return false, errors.Wrap(err, "failed to add podSpec to engine context")
		}

		value, err := ctx.Query(exclude.Path)
		if err != nil {
			return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given path %s", exclude.Path))
		}

		if !allowedValues(value, exclude) {
			return false, nil
		}
	}

	return true, nil
}

func imagesMatched(podSpec *corev1.PodSpec, images []string) bool {
	for _, container := range podSpec.Containers {
		if utils.ContainsString(images, container.Image) {
			return true
		}
	}

	return false
}

func allowedValues(resourceValue interface{}, exclude *v1.PodSecurityStandard) bool {
	addCapabilities := resourceValue.([]interface{})
	for _, capabilities := range addCapabilities {
		for _, capability := range capabilities.([]interface{}) {
			fmt.Println("====capability", exclude.Values, capability)
			if !utils.ContainsString(exclude.Values, capability.(string)) {
				return false
			}
		}
	}
	return true
}

// func getCheck(path string) policy.Check {

// }
