package autogen

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
