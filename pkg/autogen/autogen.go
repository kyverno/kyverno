package autogen

import (
	"encoding/json"
	"fmt"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
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

// GenerateRules generates rule for podControllers based on scenario A and C
func GenerateRules(spec *kyverno.Spec, controllers string, log logr.Logger) []kyverno.Rule {
	var rules []kyverno.Rule
	for _, rule := range spec.Rules {
		// handle all other controllers other than CronJob
		genRule := generateRuleForControllers(*rule.DeepCopy(), stripCronJob(controllers), log)
		if genRule != nil {
			rules = append(rules, convertRule(*genRule, "Pod"))
		}
		// handle CronJob, it appends an additional rule
		genRule = generateCronJobRule(*rule.DeepCopy(), controllers, log)
		if genRule != nil {
			rules = append(rules, convertRule(*genRule, "Cronjob"))
		}
	}
	return rules
}

func convertRule(rule kyvernoRule, kind string) kyverno.Rule {
	// TODO: marshall, rewrite and unmarshall
	if bytes, err := json.Marshal(rule); err != nil {
		// TODO
	} else {
		bytes = updateGenRuleByte(bytes, kind, rule)
		if err := json.Unmarshal(bytes, &rule); err != nil {
			// TODO
		}
	}
	out := kyverno.Rule{
		Name:         rule.Name,
		VerifyImages: rule.VerifyImages,
	}
	if rule.MatchResources != nil {
		out.MatchResources = *rule.MatchResources
	}
	if rule.ExcludeResources != nil {
		out.ExcludeResources = *rule.ExcludeResources
	}
	if rule.Context != nil {
		out.Context = *rule.Context
	}
	if rule.AnyAllConditions != nil {
		out.AnyAllConditions = *rule.AnyAllConditions
	}
	if rule.Mutation != nil {
		out.Mutation = *rule.Mutation
	}
	if rule.Validation != nil {
		out.Validation = *rule.Validation
	}
	return out
}

// MutatePolicy - applies mutation to a policy
func MutatePolicy(policy *v1.ClusterPolicy, logger logr.Logger) (*v1.ClusterPolicy, error) {
	applyAutoGen, desiredControllers := CanAutoGen(&policy.Spec, logger)

	if !applyAutoGen {
		desiredControllers = "none"
	}

	ann := policy.GetAnnotations()
	actualControllers, ok := ann[engine.PodControllersAnnotation]

	// - scenario A
	// - predefined controllers are invalid, overwrite the value
	if !ok || !applyAutoGen {
		actualControllers = desiredControllers
		// if err != nil {
		// 	errs = append(errs, fmt.Errorf("failed to generate pod controller annotation for policy '%s': %v", policy.Name, err))
		// } else {
		// 	patches = append(patches, annPatch)
		// }
	} else {
		if !applyAutoGen {
			actualControllers = desiredControllers
		}
	}

	if actualControllers == "none" {
		return policy, nil
	}

	genRules := GenerateRules(&policy.Spec, actualControllers, logger)
	if len(genRules) == 0 {
		return policy, nil
	}

	genPolicy := policy.DeepCopy()
	genPolicy.Spec.Rules = append(policy.Spec.Rules, genRules...)
	return genPolicy, nil
}
