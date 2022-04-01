package autogen

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// PodControllerCronJob represent CronJob string
	PodControllerCronJob = "CronJob"
	//PodControllers stores the list of Pod-controllers in csv string
	PodControllers = "DaemonSet,Deployment,Job,StatefulSet,CronJob"
)

func checkAutogenSupport(subjects ...kyverno.ResourceDescription) (bool, bool) {
	needed := false
	for _, subject := range subjects {
		if subject.Name != "" || subject.Selector != nil || subject.Annotations != nil {
			return false, false
		}
		if isKindOtherthanPod(subject.Kinds) {
			return false, false
		}
		needed = needed || hasAutogenKinds(subject.Kinds)
	}
	return true, needed
}

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
		if rule.Mutation.PatchesJSON6902 != "" || rule.HasGenerate() {
			return false, "none"
		}
		match, exclude := rule.MatchResources, rule.ExcludeResources
		if supported, needed := checkAutogenSupport(match.ResourceDescription, exclude.ResourceDescription); !supported {
			log.V(3).Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, ""
		} else {
			needAutogen = needAutogen || needed
		}
		for _, value := range match.Any {
			if supported, needed := checkAutogenSupport(value.ResourceDescription); !supported {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match any block is not be applicable.", "rule", rule.Name)
				return false, ""
			} else {
				needAutogen = needAutogen || needed
			}
		}
		for _, value := range match.All {
			if supported, needed := checkAutogenSupport(value.ResourceDescription); !supported {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in match all block is not be applicable.", "rule", rule.Name)
				return false, ""
			} else {
				needAutogen = needAutogen || needed
			}
		}
		for _, value := range exclude.Any {
			if supported, needed := checkAutogenSupport(value.ResourceDescription); !supported {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not be applicable.", "rule", rule.Name)
				return false, ""
			} else {
				needAutogen = needAutogen || needed
			}
		}
		for _, value := range exclude.All {
			if supported, needed := checkAutogenSupport(value.ResourceDescription); !supported {
				log.V(3).Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not be applicable.", "rule", rule.Name)
				return false, ""
			} else {
				needAutogen = needAutogen || needed
			}
		}
	}
	if !needAutogen {
		return false, ""
	}
	return true, PodControllers
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
func GetRequestedControllers(meta *metav1.ObjectMeta) []string {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		return nil
	}
	controllers, ok := annotations[kyverno.PodControllersAnnotation]
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
func GetControllers(meta *metav1.ObjectMeta, spec *kyverno.Spec, log logr.Logger) ([]string, []string, []string) {
	// compute supported and requested controllers
	supported, requested := GetSupportedControllers(spec, log), GetRequestedControllers(meta)
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
	ruleMap := createRuleMap(spec.Rules)
	var ruleIndex = make(map[string]int)
	for index, rule := range spec.Rules {
		ruleIndex[rule.Name] = index
	}
	insertIdx := len(spec.Rules)
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
			patch := jsonutils.NewPatch(fmt.Sprintf("/spec/rules/%s", strconv.Itoa(patchPostion)), operation, genRule)
			pbytes, err := patch.Marshal()
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			if err := jsonutils.CheckPatch(pbytes); err != nil {
				errs = append(errs, err)
				return nil
			}
			return pbytes
		}
		// handle all other controllers other than CronJob
		genRule := createRule(generateRuleForControllers(rule, stripCronJob(controllers), log))
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
		genRule = createRule(generateCronJobRule(rule, controllers, log))
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

// podControllersKey annotation could be:
// scenario A: not exist, set default to "all", which generates on all pod controllers
//               - if name / selector exist in resource description -> skip
//                 as these fields may not be applicable to pod controllers
// scenario B: "none", user explicitly disable this feature -> skip
// scenario C: some certain controllers that user set -> generate on defined controllers
//             copy entire match / exclude block, it's users' responsibility to
//             make sure all fields are applicable to pod controllers

// GenerateRules generates rule for podControllers based on scenario A and C
func GenerateRules(spec *kyverno.Spec, controllers string, log logr.Logger) []kyverno.Rule {
	var rules []kyverno.Rule
	for _, rule := range spec.Rules {
		// handle all other controllers other than CronJob
		if genRule := createRule(generateRuleForControllers(*rule.DeepCopy(), stripCronJob(controllers), log)); genRule != nil {
			if convRule, err := convertRule(*genRule, "Pod"); err == nil {
				rules = append(rules, *convRule)
			}
		}
		// handle CronJob, it appends an additional rule
		if genRule := createRule(generateCronJobRule(*rule.DeepCopy(), controllers, log)); genRule != nil {
			if convRule, err := convertRule(*genRule, "Cronjob"); err == nil {
				rules = append(rules, *convRule)
			}
		}
	}
	return rules
}

func convertRule(rule kyvernoRule, kind string) (*kyverno.Rule, error) {
	if bytes, err := json.Marshal(rule); err != nil {
		return nil, err
	} else {
		bytes = updateGenRuleByte(bytes, kind, rule)
		if err := json.Unmarshal(bytes, &rule); err != nil {
			return nil, err
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
		out.SetAnyAllConditions(*rule.AnyAllConditions)
	}
	if rule.Mutation != nil {
		out.Mutation = *rule.Mutation
	}
	if rule.Validation != nil {
		out.Validation = *rule.Validation
	}
	return &out, nil
}

func ComputeRules(p kyverno.PolicyInterface) []kyverno.Rule {
	spec := p.GetSpec()
	if !toggle.AutogenInternals() {
		return spec.Rules
	}
	applyAutoGen, desiredControllers := CanAutoGen(spec, log.Log)
	if !applyAutoGen {
		desiredControllers = "none"
	}
	ann := p.GetAnnotations()
	actualControllers, ok := ann[kyverno.PodControllersAnnotation]
	if !ok || !applyAutoGen {
		actualControllers = desiredControllers
	} else {
		if !applyAutoGen {
			actualControllers = desiredControllers
		}
	}
	if actualControllers == "none" {
		return spec.Rules
	}
	genRules := GenerateRules(spec.DeepCopy(), actualControllers, log.Log)
	if len(genRules) == 0 {
		return spec.Rules
	}
	var out []kyverno.Rule
	out = append(out, spec.Rules...)
	out = append(out, genRules...)
	return out
}
