package policy

import (
	"context"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	authChecker "github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/policy/mutate"
	"github.com/kyverno/kyverno/pkg/policy/validate"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
)

// Validation provides methods to validate a rule
type Validation interface {
	Validate(ctx context.Context, verbs []string) (warnings []string, path string, err error)
}

// validateAction performs validation on the rule actions
// - Mutate
// - Validation
// - Generate
func validateActions(idx int, rule *kyvernov1.Rule, client dclient.Interface, mock bool, backgroundSA, reportsSA string) (warnings []string, err error) {
	if rule == nil {
		return nil, nil
	}

	var checker Validation

	// Mutate
	if rule.HasMutate() {
		checker = mutate.NewMutateFactory(rule, client, mock, backgroundSA, reportsSA)
		if w, path, err := checker.Validate(context.TODO(), nil); err != nil {
			return nil, fmt.Errorf("path: spec.rules[%d].mutate.%s.: %v", idx, path, err)
		} else if w != nil {
			warnings = append(warnings, w...)
		}
	}

	// Validate
	if rule.HasValidate() {
		checker = validate.NewValidateFactory(rule, client, mock, reportsSA)
		if w, path, err := checker.Validate(context.TODO(), nil); err != nil {
			return nil, fmt.Errorf("path: spec.rules[%d].validate.%s.: %v", idx, path, err)
		} else if w != nil {
			warnings = append(warnings, w...)
		}

		if rule.HasValidateCEL() && toggle.FromContext(context.TODO()).GenerateValidatingAdmissionPolicy() {
			authCheck := authChecker.NewSelfChecker(client.GetKubeClient().AuthorizationV1().SelfSubjectAccessReviews())
			if !validatingadmissionpolicy.HasValidatingAdmissionPolicyPermission(authCheck) {
				warnings = append(warnings, "insufficient permissions to generate ValidatingAdmissionPolicies")
			}

			if !validatingadmissionpolicy.HasValidatingAdmissionPolicyBindingPermission(authCheck) {
				warnings = append(warnings, "insufficient permissions to generate ValidatingAdmissionPolicies")
			}
		}
	}

	// Generate
	if rule.HasGenerate() {
		// TODO: this check is there to support offline validations
		// generate uses selfSubjectReviews to verify actions
		// this need to modified to use different implementation for online and offline mode
		if mock {
			checker = generate.NewFakeGenerate(*rule.Generation)
			if w, path, err := checker.Validate(context.TODO(), nil); err != nil {
				return nil, fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			} else if warnings != nil {
				warnings = append(warnings, w...)
			}
		} else {
			if rule.Generation.Synchronize {
				admissionSA := fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName())
				checker = generate.NewGenerateFactory(client, rule, admissionSA, reportsSA, logging.GlobalLogger())
				if w, path, err := checker.Validate(context.TODO(), []string{"list", "get"}); err != nil {
					return nil, fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
				} else if warnings != nil {
					warnings = append(warnings, w...)
				}
			}
			checker = generate.NewGenerateFactory(client, rule, backgroundSA, reportsSA, logging.GlobalLogger())
			if w, path, err := checker.Validate(context.TODO(), nil); err != nil {
				return nil, fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			} else if warnings != nil {
				warnings = append(warnings, w...)
			}
		}

		if slices.Contains(rule.MatchResources.Kinds, rule.Generation.Kind) {
			return nil, fmt.Errorf("generation kind and match resource kind should not be the same")
		}
	}

	return warnings, nil
}
