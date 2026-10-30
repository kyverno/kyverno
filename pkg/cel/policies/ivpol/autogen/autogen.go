package autogen

import (
	"cmp"
	"maps"
	"slices"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/autogen"
	"k8s.io/apimachinery/pkg/util/sets"
)

// RewriteExceptions rewrites PolicyException match conditions and validations using the same
// spec field-path replacements applied to the policy spec for the given autogen
// config, so exceptions match against the autogen'd controller's shape
// instead of the original Pod-shaped policy.
func RewriteExceptions(exceptions []*policiesv1beta1.PolicyException, config string) ([]*policiesv1beta1.PolicyException, error) {
	replacements := slices.DeleteFunc(slices.Clone(autogen.ReplacementsMap[config]), func(r autogen.Replacement) bool {
		return r.From == "metadata"
	})
	if len(replacements) == 0 || len(exceptions) == 0 {
		return exceptions, nil
	}

	out := make([]*policiesv1beta1.PolicyException, 0, len(exceptions))
	for _, polex := range exceptions {
		polex := polex.DeepCopy()

		for i := range polex.Spec.MatchConditions {
			expr := polex.Spec.MatchConditions[i].Expression
			if expr != "" {
				polex.Spec.MatchConditions[i].Expression = string(autogen.ApplyCEL([]byte(expr), replacements...))
			}
		}

		for i := range polex.Spec.Validations {
			expr := polex.Spec.Validations[i].Expression
			if expr != "" {
				polex.Spec.Validations[i].Expression = string(autogen.ApplyCEL([]byte(expr), replacements...))
			}
			msgExpr := polex.Spec.Validations[i].MessageExpression
			if msgExpr != "" {
				polex.Spec.Validations[i].MessageExpression = string(autogen.ApplyCEL([]byte(msgExpr), replacements...))
			}
		}

		out = append(out, polex)
	}
	return out, nil
}

// ExtractionReplacementsRef re-exports autogen.ExtractionReplacementsRef so
// callers of this package (e.g. the engine provider) don't need to import
// pkg/cel/autogen directly just to check it.
const ExtractionReplacementsRef = autogen.ExtractionReplacementsRef

func Autogen(policy policiesv1beta1.ImageValidatingPolicyLike) (map[string]policiesv1beta1.ImageValidatingPolicyAutogen, error) {
	if policy == nil {
		return nil, nil
	}
	spec := policy.GetSpec()
	if !autogen.CanAutoGen(spec.MatchConstraints) {
		return nil, nil
	}
	actualControllers := autogen.AllConfigs
	if spec.AutogenConfiguration != nil && spec.AutogenConfiguration.PodControllers != nil {
		pc := spec.AutogenConfiguration.PodControllers
		if pc.Enabled != nil && !*pc.Enabled {
			return nil, nil
		}
		if pc.Controllers != nil {
			actualControllers = sets.New(pc.Controllers...)
		}
	}
	return autogenIvPols(*spec, actualControllers)
}

func autogenIvPols(spec policiesv1beta1.ImageValidatingPolicySpec, configs sets.Set[string]) (map[string]policiesv1beta1.ImageValidatingPolicyAutogen, error) {
	mapping := map[string][]policiesv1beta1.Target{}
	for config := range configs {
		if config := autogen.ResolveConfig(config); config != nil {
			targets := mapping[config.ReplacementsRef]
			targets = append(targets, config.Target)
			mapping[config.ReplacementsRef] = targets
		}
	}
	rules := map[string]policiesv1beta1.ImageValidatingPolicyAutogen{}
	for _, config := range slices.Sorted(maps.Keys(mapping)) {
		targets := mapping[config]
		spec := spec.DeepCopy()
		operations := spec.MatchConstraints.ResourceRules[0].Operations
		spec.MatchConstraints = autogen.CreateMatchConstraints(targets, operations, spec.MatchConstraints.NamespaceSelector)
		replacements := autogen.ReplacementsMap[config]
		if len(replacements) > 0 {
			applyCEL := func(expr string) string {
				if expr == "" {
					return ""
				}
				return string(autogen.ApplyCEL([]byte(expr), replacements...))
			}
			for i := range spec.MatchConditions {
				spec.MatchConditions[i].Expression = applyCEL(spec.MatchConditions[i].Expression)
			}
			for i := range spec.Variables {
				spec.Variables[i].Expression = applyCEL(spec.Variables[i].Expression)
			}
			for i := range spec.Validations {
				spec.Validations[i].Expression = applyCEL(spec.Validations[i].Expression)
				spec.Validations[i].MessageExpression = applyCEL(spec.Validations[i].MessageExpression)
			}
			for i := range spec.AuditAnnotations {
				spec.AuditAnnotations[i].ValueExpression = applyCEL(spec.AuditAnnotations[i].ValueExpression)
			}
			for i := range spec.MatchImageReferences {
				spec.MatchImageReferences[i].Expression = applyCEL(spec.MatchImageReferences[i].Expression)
			}
			for i := range spec.ImageExtractors {
				spec.ImageExtractors[i].Expression = applyCEL(spec.ImageExtractors[i].Expression)
			}
			for i := range spec.Attestors {
				if cosign := spec.Attestors[i].Cosign; cosign != nil {
					if cosign.Key != nil {
						cosign.Key.Expression = applyCEL(cosign.Key.Expression)
					}
					if cosign.Keyless != nil {
						for j := range cosign.Keyless.Identities {
							cosign.Keyless.Identities[j].SubjectExpression = applyCEL(cosign.Keyless.Identities[j].SubjectExpression)
						}
					}
					if cosign.Certificate != nil {
						if cosign.Certificate.Certificate != nil {
							cosign.Certificate.Certificate.Expression = applyCEL(cosign.Certificate.Certificate.Expression)
						}
						if cosign.Certificate.CertificateChain != nil {
							cosign.Certificate.CertificateChain.Expression = applyCEL(cosign.Certificate.CertificateChain.Expression)
						}
					}
					if cosign.TrustedRoot != nil {
						cosign.TrustedRoot.Expression = applyCEL(cosign.TrustedRoot.Expression)
					}
				}
				if notary := spec.Attestors[i].Notary; notary != nil {
					if notary.Certs != nil {
						notary.Certs.Expression = applyCEL(notary.Certs.Expression)
					}
					if notary.TSACerts != nil {
						notary.TSACerts.Expression = applyCEL(notary.TSACerts.Expression)
					}
				}
			}
		}
		slices.SortFunc(targets, func(a, b policiesv1beta1.Target) int {
			if x := cmp.Compare(a.Group, b.Group); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Version, b.Version); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Resource, b.Resource); x != 0 {
				return x
			}
			if x := cmp.Compare(a.Kind, b.Kind); x != 0 {
				return x
			}
			return 0
		})
		rules[config] = policiesv1beta1.ImageValidatingPolicyAutogen{
			Targets: targets,
			Spec:    spec,
		}
	}
	return rules, nil
}
