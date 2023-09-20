package autogen

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// PodControllerCronJob represent CronJob string
	PodControllerCronJob = "CronJob"
	// PodControllers stores the list of Pod-controllers in csv string
	PodControllers = "DaemonSet,Deployment,Job,StatefulSet,ReplicaSet,ReplicationController,CronJob"
)

var podControllersKindsSet = sets.New(append(strings.Split(PodControllers, ","), "Pod")...)

func isKindOtherthanPod(kinds []string) bool {
	if len(kinds) > 1 && kubeutils.ContainsKind(kinds, "Pod") {
		return true
	}
	return false
}

func checkAutogenSupport(needed *bool, subjects ...kyvernov1.ResourceDescription) bool {
	for _, subject := range subjects {
		if subject.Name != "" || len(subject.Names) > 0 || subject.Selector != nil || subject.Annotations != nil || isKindOtherthanPod(subject.Kinds) {
			return false
		}
		if needed != nil {
			*needed = *needed || podControllersKindsSet.HasAny(subject.Kinds...)
		}
	}
	return true
}

// stripCronJob removes CronJob from controllers
func stripCronJob(controllers string) string {
	var newControllers []string
	controllerArr := strings.Split(controllers, ",")
	for _, c := range controllerArr {
		if c == PodControllerCronJob {
			continue
		}
		newControllers = append(newControllers, c)
	}
	if len(newControllers) == 0 {
		return ""
	}
	return strings.Join(newControllers, ",")
}

// CanAutoGen checks whether the rule(s) (in policy) can be applied to Pod controllers
// returns controllers as:
// - "" if:
//   - name or selector is defined
//   - mixed kinds (Pod + pod controller) is defined
//   - Pod and PodControllers are not defined
//   - mutate.Patches/mutate.PatchesJSON6902/validate.deny/generate rule is defined
//
// - otherwise it returns all pod controllers
func CanAutoGen(spec *kyvernov1.Spec) (applyAutoGen bool, controllers string) {
	needed := false
	for _, rule := range spec.Rules {
		if rule.Mutation.PatchesJSON6902 != "" || rule.HasGenerate() {
			return false, "none"
		}
		for _, foreach := range rule.Mutation.ForEachMutation {
			if foreach.PatchesJSON6902 != "" {
				return false, "none"
			}
		}
		match, exclude := rule.MatchResources, rule.ExcludeResources
		if !checkAutogenSupport(&needed, match.ResourceDescription, exclude.ResourceDescription) {
			debug.Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, ""
		}
		for _, value := range match.Any {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match any block is not applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range match.All {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match all block is not applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range exclude.Any {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not applicable.", "rule", rule.Name)
				return false, ""
			}
		}
		for _, value := range exclude.All {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not applicable.", "rule", rule.Name)
				return false, ""
			}
		}
	}
	if !needed {
		return false, ""
	}
	return true, PodControllers
}

// GetSupportedControllers returns the supported autogen controllers for a given spec.
func GetSupportedControllers(spec *kyvernov1.Spec) []string {
	apply, controllers := CanAutoGen(spec)
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
	controllers, ok := annotations[kyverno.AnnotationAutogenControllers]
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
func GetControllers(meta *metav1.ObjectMeta, spec *kyvernov1.Spec) ([]string, []string, []string) {
	// compute supported and requested controllers
	supported, requested := GetSupportedControllers(spec), GetRequestedControllers(meta)
	// no specific request, we can return supported controllers without further filtering
	if requested == nil {
		return requested, supported, supported
	}
	// filter supported controllers, keeping only those that have been requested
	var activated []string
	for _, controller := range supported {
		if slices.Contains(requested, controller) {
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

// generateRules generates rule for podControllers based on scenario A and C
func generateRules(spec *kyvernov1.Spec, controllers string) []kyvernov1.Rule {
	var rules []kyvernov1.Rule
	for i := range spec.Rules {
		// handle all other controllers other than CronJob
		if genRule := createRule(generateRuleForControllers(&spec.Rules[i], stripCronJob(controllers))); genRule != nil {
			if convRule, err := convertRule(*genRule, "Pod"); err == nil {
				rules = append(rules, *convRule)
			} else {
				logger.Error(err, "failed to create rule")
			}
		}
		// handle CronJob, it appends an additional rule
		if genRule := createRule(generateCronJobRule(&spec.Rules[i], controllers)); genRule != nil {
			if convRule, err := convertRule(*genRule, "Cronjob"); err == nil {
				rules = append(rules, *convRule)
			} else {
				logger.Error(err, "failed to create Cronjob rule")
			}
		}
	}
	return rules
}

func convertRule(rule kyvernoRule, kind string) (*kyvernov1.Rule, error) {
	if bytes, err := json.Marshal(rule); err != nil {
		return nil, err
	} else {
		if rule.Validation != nil && rule.Validation.PodSecurity != nil {
			bytes = updateRestrictedFields(bytes, kind)
			if err := json.Unmarshal(bytes, &rule); err != nil {
				return nil, err
			}
		} else {
			bytes = updateGenRuleByte(bytes, kind)
			if err := json.Unmarshal(bytes, &rule); err != nil {
				return nil, err
			}
		}

		// CEL variables are object, oldObject, request, params and authorizer.
		// Therefore CEL expressions can be either written as object.spec or request.object.spec
		if rule.Validation != nil && rule.Validation.CEL != nil {
			bytes = updateCELFields(bytes, kind)
			if err := json.Unmarshal(bytes, &rule); err != nil {
				return nil, err
			}
		}
	}

	out := kyvernov1.Rule{
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

func ComputeRules(p kyvernov1.PolicyInterface) []kyvernov1.Rule {
	return computeRules(p)
}

func computeRules(p kyvernov1.PolicyInterface) []kyvernov1.Rule {
	spec := p.GetSpec()
	applyAutoGen, desiredControllers := CanAutoGen(spec)
	if !applyAutoGen {
		desiredControllers = "none"
	}
	ann := p.GetAnnotations()
	actualControllers, ok := ann[kyverno.AnnotationAutogenControllers]
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
	genRules := generateRules(spec.DeepCopy(), actualControllers)
	if len(genRules) == 0 {
		return spec.Rules
	}
	var out []kyvernov1.Rule
	for _, rule := range spec.Rules {
		if !isAutogenRuleName(rule.Name) {
			out = append(out, rule)
		}
	}
	out = append(out, genRules...)
	return out
}
