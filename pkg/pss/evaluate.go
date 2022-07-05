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

func EvaluatePSS(lv api.LevelVersion, podMetadata *metav1.ObjectMeta, podSpec *corev1.PodSpec) (results []policy.CheckResult) {
	checks := policy.DefaultChecks()

	for _, check := range checks {

		// Restricted ? Baseline + Restricted (cumulative)
		// Baseline ? Then ignore checks for Restricted
		// fmt.Printf("current level: %s, check level: %s\n", lv.Level, check.Level)
		if lv.Level == api.LevelBaseline && check.Level != lv.Level {
			continue
		}

		// check version

		for _, versionCheck := range check.Versions {
			res := versionCheck.CheckPod(podMetadata, podSpec)
			// fmt.Printf("%v, res: %v\n", versionCheck, res)
			if !res.Allowed {
				fmt.Printf("check error: %v\n", res)
				results = append(results, res)
			}
		}
	}
	return
}

func ExemptProfile(rule *v1.PodSecurity, podSpec *corev1.PodSpec, podObjectMeta *metav1.ObjectMeta) (bool, error) {
	ctx := enginectx.NewContext()

	for _, exclude := range rule.Exclude {
		if !imagesMatched(podSpec, exclude.Images) {
			continue
		}

		// double check if the given RestrictedField violates the specific profile?
		// need a RestrictedField - check ID map to fetch psa Check

		if podObjectMeta != nil {
			if err := ctx.AddJSONObject(podObjectMeta); err != nil {
				return false, errors.Wrap(err, "failed to add podObjectMeta to engine context")
			}
		} else {
			if err := ctx.AddJSONObject(podSpec); err != nil {
				return false, errors.Wrap(err, "failed to add podSpec to engine context")
			}
		}

		fmt.Printf("Restricted Field: %v\n", exclude.RestrictedField)
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
	// When resourceValue is a bool (Host Namespaces control)
	if reflect.TypeOf(resourceValue).Kind() == reflect.Bool {
		fmt.Printf("exclude values %v,  resourceValue: %v\n", exclude.Values, resourceValue)
		if !utils.ContainsString(exclude.Values, strconv.FormatBool(resourceValue.(bool))) {
			return false
		}
		return true
	}

	excludeValues := resourceValue.([]interface{})

	fmt.Println(excludeValues)

	for k, values := range excludeValues {
		rt := reflect.TypeOf(values)
		kind := rt.Kind()

		if kind == reflect.Slice {
			fmt.Println(k, "is a slice with element type", rt.Elem())
			for _, value := range values.([]interface{}) {
				fmt.Printf("value: %s\n", value)
				fmt.Printf("exclude values %v,  value: %s\n", exclude.Values, value)
				if !utils.ContainsString(exclude.Values, value.(string)) {
					return false
				}
			}
		} else if kind == reflect.Array {
			fmt.Println(k, "is an array with element type", rt.Elem())
		} else if kind == reflect.Map {
			// For Volume Types control
			fmt.Println("is a map with element type", rt.Elem())
			for key, value := range values.(map[string]interface{}) {
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
