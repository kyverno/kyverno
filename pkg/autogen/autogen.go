package autogen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CanAutoGen checks whether the rule(s) (in policy) can be applied to Pod controllers
// returns controllers as:
// - "" if:
//          - name or selector is defined
//          - mixed kinds (Pod + pod controller) is defined
//          - Pod and PodControllers are not defined
//          - mutate.Patches/mutate.PatchesJSON6902/validate.deny/generate rule is defined
// - otherwise it returns all pod controllers
func CanAutoGen(spec *kyverno.Spec, log logr.Logger) (applyAutoGen bool, controllers string) {
	var needAutogen bool
	for _, rule := range spec.Rules {
		match := rule.MatchResources
		exclude := rule.ExcludeResources

		if match.ResourceDescription.Name != "" || match.ResourceDescription.Selector != nil || match.ResourceDescription.Annotations != nil ||
			exclude.ResourceDescription.Name != "" || exclude.ResourceDescription.Selector != nil || exclude.ResourceDescription.Annotations != nil {
			log.V(3).Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, ""
		}

		if isKindOtherthanPod(match.Kinds) || isKindOtherthanPod(exclude.Kinds) {
			return false, ""
		}

		needAutogen = hasAutogenKinds(match.Kinds) || hasAutogenKinds(exclude.Kinds)

		for _, value := range match.Any {
			if isKindOtherthanPod(value.Kinds) {
				return false, ""
			}
			if !needAutogen {
				needAutogen = hasAutogenKinds(value.Kinds)
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match any block is not be applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range match.All {
			if isKindOtherthanPod(value.Kinds) {
				return false, ""
			}
			if !needAutogen {
				needAutogen = hasAutogenKinds(value.Kinds)
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match all block is not be applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range exclude.Any {
			if isKindOtherthanPod(value.Kinds) {
				return false, ""
			}
			if !needAutogen {
				needAutogen = hasAutogenKinds(value.Kinds)
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not be applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range exclude.All {
			if isKindOtherthanPod(value.Kinds) {
				return false, ""
			}
			if !needAutogen {
				needAutogen = hasAutogenKinds(value.Kinds)
			}
			if value.Name != "" || value.Selector != nil || value.Annotations != nil {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not be applicable.", "rule", rule.Name)
				return false, ""
			}
		}

		if rule.Mutation.PatchesJSON6902 != "" || rule.HasGenerate() {
			return false, "none"
		}
	}

	if !needAutogen {
		return false, ""
	}

	return true, engine.PodControllers
}

// GetSupportedControllers returns the supported autogen controllers for a given spec.
func GetSupportedControllers(spec *kyverno.Spec, log logr.Logger) []string {
	apply, controllers := CanAutoGen(spec, log)
	if !apply || controllers == "none" {
		return nil
	}
	return strings.Split(controllers, ",")
}

// GetRequestedControllers returns the requested autogen controllers based on object annotations.
func GetRequestedControllers(meta metav1.ObjectMeta) []string {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		return nil
	}
	controllers, ok := annotations[engine.PodControllersAnnotation]
	if !ok || controllers == "" {
		return nil
	}
	if controllers == "none" {
		return []string{}
	}
	return strings.Split(controllers, ",")
}

// GetControllers computes the autogen controllers that should be applied to a policy.
// It returns the requested, supported and effective controllers (intersection of requested and supported ones).
func GetControllers(meta metav1.ObjectMeta, spec *kyverno.Spec, log logr.Logger) ([]string, []string, []string) {
	// compute supported and requested controllers
	supported := GetSupportedControllers(spec, log)
	requested := GetRequestedControllers(meta)

	// no specific request, we can return supported controllers without further filtering
	if requested == nil {
		return requested, supported, supported
	}

	// filter supported controllers, keeping only those that have been requested
	var activated []string
	for _, controller := range supported {
		if utils.ContainsString(requested, controller) {
			activated = append(activated, controller)
		}
	}
	return requested, supported, activated
}

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "none", user explicitly disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entire match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod controllers

// GenerateRulePatches generates rule for podControllers based on scenario A and C
func GenerateRulePatches(spec *kyverno.Spec, controllers string, log logr.Logger) (rulePatches [][]byte, errs []error) {
	insertIdx := len(spec.Rules)

	ruleMap := createRuleMap(spec.Rules)
	var ruleIndex = make(map[string]int)
	for index, rule := range spec.Rules {
		ruleIndex[rule.Name] = index
	}

	for _, rule := range spec.Rules {
		patchPostion := insertIdx
		convertToPatches := func(genRule kyvernoRule, patchPostion int) []byte {
			operation := "add"
			if existingAutoGenRule, alreadyExists := ruleMap[genRule.Name]; alreadyExists {
				existingAutoGenRuleRaw, _ := json.Marshal(existingAutoGenRule)
				genRuleRaw, _ := json.Marshal(genRule)

				if string(existingAutoGenRuleRaw) == string(genRuleRaw) {
					return nil
				}
				operation = "replace"
				patchPostion = ruleIndex[genRule.Name]
			}

			// generate patch bytes
			jsonPatch := struct {
				Path  string      `json:"path"`
				Op    string      `json:"op"`
				Value interface{} `json:"value"`
			}{
				fmt.Sprintf("/spec/rules/%s", strconv.Itoa(patchPostion)),
				operation,
				genRule,
			}
			pbytes, err := json.Marshal(jsonPatch)
			if err != nil {
				errs = append(errs, err)
				return nil
			}

			// check the patch
			if _, err := jsonpatch.DecodePatch([]byte("[" + string(pbytes) + "]")); err != nil {
				errs = append(errs, err)
				return nil
			}

			return pbytes
		}

		// handle all other controllers other than CronJob
		genRule := generateRuleForControllers(rule, stripCronJob(controllers), log)
		if genRule != nil {
			pbytes := convertToPatches(*genRule, patchPostion)
			pbytes = updateGenRuleByte(pbytes, "Pod", *genRule)
			if pbytes != nil {
				rulePatches = append(rulePatches, pbytes)
			}
			insertIdx++
			patchPostion = insertIdx
		}

		// handle CronJob, it appends an additional rule
		genRule = generateCronJobRule(rule, controllers, log)
		if genRule != nil {
			pbytes := convertToPatches(*genRule, patchPostion)
			pbytes = updateGenRuleByte(pbytes, "Cronjob", *genRule)
			if pbytes != nil {
				rulePatches = append(rulePatches, pbytes)
			}
			insertIdx++
		}
	}
	return
}
