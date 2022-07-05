package pss

import (
	"fmt"
	"reflect"

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
	fmt.Println("ExemptProfile")
	ctx := enginectx.NewContext()

	for _, exclude := range rule.Exclude {
		if !imagesMatched(podSpec, exclude.Images) {
			continue
		}

		// double check if the given RestrictedField violates the specific profile?
		// need a RestrictedField - check ID map to fetch psa Check

		if err := ctx.AddJSONObject(podSpec); err != nil {
			return false, errors.Wrap(err, "failed to add podSpec to engine context")
		}

		value, err := ctx.Query(exclude.RestrictedField)
		if err != nil {
			return false, errors.Wrap(err, fmt.Sprintf("failed to query value with the given RestrictedField %s", exclude.RestrictedField))
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
	fmt.Println("allowedValues")
	fmt.Println(addCapabilities)

	for k, capabilities := range addCapabilities {
		rt := reflect.TypeOf(capabilities)
		kind := rt.Kind()

		if kind == reflect.Slice {
			fmt.Println(k, "is a slice with element type", rt.Elem())
			for _, capability := range capabilities.([]interface{}) {
				fmt.Printf("value: %s\n", capability)
				fmt.Println("====capability", exclude.Values, capability)
				if !utils.ContainsString(exclude.Values, capability.(string)) {
					return false
				}
			}
		} else if kind == reflect.Array {
			fmt.Println(k, "is an array with element type", rt.Elem())
		} else if kind == reflect.Map {
			// For Volume Types control
			fmt.Println("is a map with element type", rt.Elem())
			for key, value := range capabilities.(map[string]interface{}) {
				// `Volume`` has 2 fields: `Name` and a `Volume Source` (inline json)
				// Ignore `Name` field because we want to look at `Volume Source`'s key
				// https://github.com/kubernetes/api/blob/f18d381b8d0129e7098e1e67a89a8088f2dba7e6/core/v1/types.go#L36
				if key == "name" {
					continue
				}
				fmt.Printf("exclude values %v, key: %s, value: %s\n", exclude.Values, key, value)
				if !utils.ContainsString(exclude.Values, key) {
					return false
				}
			}
		} else {
			fmt.Println(k, "is something else entirely")
		}
	}
	return true
}

// func getCheck(path string) policy.Check {

// }
