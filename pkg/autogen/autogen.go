package autogen

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// PodControllerCronJob represent CronJob string
	PodControllerCronJob = "CronJob"
)

var (
	PodControllers         = sets.New("DaemonSet", "Deployment", "Job", "StatefulSet", "ReplicaSet", "ReplicationController", "CronJob")
	podControllersKindsSet = PodControllers.Union(sets.New("Pod"))
	assertAutogenNodes     = []string{"object", "oldObject"}
)

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
	controllerArr := splitKinds(controllers, ",")
	newControllers := make([]string, 0, len(controllerArr))
	for _, c := range controllerArr {
		if c == PodControllerCronJob {
			continue
		}
		newControllers = append(newControllers, c)
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
func CanAutoGen(spec *kyvernov1.Spec) (applyAutoGen bool, controllers sets.Set[string]) {
	needed := false
	for _, rule := range spec.Rules {
		if rule.Mutation.PatchesJSON6902 != "" || rule.HasGenerate() {
			return false, sets.New("none")
		}
		for _, foreach := range rule.Mutation.ForEachMutation {
			if foreach.PatchesJSON6902 != "" {
				return false, sets.New("none")
			}
		}
		match, exclude := rule.MatchResources, rule.ExcludeResources
		if !checkAutogenSupport(&needed, match.ResourceDescription, exclude.ResourceDescription) {
			debug.Info("skip generating rule on pod controllers: Name / Selector in resource description may not be applicable.", "rule", rule.Name)
			return false, sets.New[string]()
		}
		for _, value := range match.Any {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match any block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
		for _, value := range match.All {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in match all block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
		for _, value := range exclude.Any {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in exclude any block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
		for _, value := range exclude.All {
			if !checkAutogenSupport(&needed, value.ResourceDescription) {
				debug.Info("skip generating rule on pod controllers: Name / Selector in exclud all block is not applicable.", "rule", rule.Name)
				return false, sets.New[string]()
			}
		}
	}
	if !needed {
		return false, sets.New[string]()
	}
	return true, PodControllers
}

// GetSupportedControllers returns the supported autogen controllers for a given spec.
func GetSupportedControllers(spec *kyvernov1.Spec) sets.Set[string] {
	apply, controllers := CanAutoGen(spec)
	if !apply || (controllers.Len() == 1 && controllers.Has("none")) {
		return nil
	}
	return controllers
}

// GetRequestedControllers returns the requested autogen controllers based on object annotations.
func GetRequestedControllers(meta *metav1.ObjectMeta) sets.Set[string] {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		return nil
	}
	controllers, ok := annotations[kyverno.AnnotationAutogenControllers]
	if !ok || controllers == "" {
		return nil
	}
	if controllers == "none" {
		return sets.New[string]()
	}
	return sets.New(splitKinds(controllers, ",")...)
}

// GetControllers computes the autogen controllers that should be applied to a policy.
// It returns the requested, supported and effective controllers (intersection of requested and supported ones).
func GetControllers(meta *metav1.ObjectMeta, spec *kyvernov1.Spec) ([]string, []string, []string) {
	// compute supported and requested controllers
	supported, requested := GetSupportedControllers(spec), GetRequestedControllers(meta)
	// no specific request, we can return supported controllers without further filtering
	if requested == nil {
		return requested.UnsortedList(), supported.UnsortedList(), supported.UnsortedList()
	}
	// filter supported controllers, keeping only those that have been requested
	var activated []string
	for _, controller := range supported.UnsortedList() {
		if requested.Has(controller) {
			activated = append(activated, controller)
		}
	}
	return requested.UnsortedList(), supported.UnsortedList(), activated
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
	var wg sync.WaitGroup
	rules := make([]kyvernov1.Rule, 0, len(spec.Rules)*2)
	resultChan := make(chan kyvernov1.Rule)
	controllersStripCronJob := stripCronJob(controllers)

	for i := range spec.Rules {
		wg.Add(2)

		// handle all other controllers other than CronJob
		go func(index int) {
			defer wg.Done()
			processRule(spec, index, controllersStripCronJob, "Pod", resultChan)
		}(i)

		// handle CronJob, it appends an additional rule
		go func(index int) {
			defer wg.Done()
			processRule(spec, index, controllers, "Cronjob", resultChan)
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for rule := range resultChan {
		rules = append(rules, rule)
	}
	return rules
}

func processRule(spec *kyvernov1.Spec, index int, controllers, kind string, result chan<- kyvernov1.Rule) {
	var genRule *kyvernoRule

	if kind == "Pod" {
		genRule = createRule(generateRuleForControllers(&spec.Rules[index], controllers))
	} else if kind == "Cronjob" {
		genRule = createRule(generateCronJobRule(&spec.Rules[index], controllers))
	}

	if genRule != nil {
		convRule, err := convertRule(*genRule, kind)
		if err == nil {
			result <- *convRule
		} else {
			logger.Error(err, fmt.Sprintf("failed to create %s rule", kind))
		}
	}
}

func convertRule(rule kyvernoRule, kind string) (*kyvernov1.Rule, error) {
	if bytes, err := json.Marshal(rule); err != nil {
		return nil, err
	} else {
		// CEL variables are object, oldObject, request, params and authorizer.
		// Therefore CEL expressions can be either written as object.spec or request.object.spec
		bytes = updateFields(bytes, kind, rule.Validation != nil && rule.Validation.CEL != nil)
		if err := json.Unmarshal(bytes, &rule); err != nil {
			return nil, err
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
		out.SetAnyAllConditions(rule.AnyAllConditions.Conditions)
	}
	if rule.Mutation != nil {
		out.Mutation = *rule.Mutation
	}
	if rule.Validation != nil {
		out.Validation = *rule.Validation
	}
	return &out, nil
}

func ComputeRules(p kyvernov1.PolicyInterface, kind string) []kyvernov1.Rule {
	return computeRules(p, kind)
}

func computeRules(p kyvernov1.PolicyInterface, kind string) []kyvernov1.Rule {
	spec := p.GetSpec()

	var actualControllers sets.Set[string]
	if applyAutoGen, desiredControllers := CanAutoGen(spec); !applyAutoGen {
		actualControllers = sets.New("none")
	} else {
		ann := p.GetAnnotations()
		actualControllersString, ok := ann[kyverno.AnnotationAutogenControllers]
		if !ok {
			actualControllers = desiredControllers
		} else {
			actualControllers = sets.New(strings.Split(actualControllersString, ",")...)
		}
	}

	if kind != "" {
		if !actualControllers.Has(kind) {
			return spec.Rules
		}
	} else {
		kind = strings.Join(actualControllers.UnsortedList(), ",")
	}

	if kind == "none" {
		return spec.Rules
	}

	genRules := generateRules(spec.DeepCopy(), kind)
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

func copyMap(m map[string]any) map[string]any {
	newMap := make(map[string]any, len(m))
	for k, v := range m {
		newMap[k] = v
	}

	return newMap
}

func createAutogenAssertion(tree kyvernov1.AssertionTree, tplKey string) kyvernov1.AssertionTree {
	v, ok := tree.Value.(map[string]any)
	if !ok {
		return tree
	}

	value := copyMap(v)

	for _, n := range assertAutogenNodes {
		object, ok := v[n].(map[string]any)
		if !ok {
			continue
		}

		value[n] = map[string]any{
			"spec": map[string]any{
				tplKey: copyMap(object),
			},
		}
	}

	return kyvernov1.AssertionTree{
		Value: value,
	}
}
