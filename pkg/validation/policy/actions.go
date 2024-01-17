package policy

import (
	"context"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	authChecker "github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/policy/mutate"
	"github.com/kyverno/kyverno/pkg/policy/validate"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils/validatingadmissionpolicy"
)

// Validation provides methods to validate a rule
type Validation interface {
	Validate(ctx context.Context) (string, error)
}

// validateAction performs validation on the rule actions
// - Mutate
// - Validation
// - Generate
func validateActions(idx int, rule *kyvernov1.Rule, client dclient.Interface, mock bool, username string) (string, error) {
	if rule == nil {
		return "", nil
	}

	var checker Validation
	// Mutate
	if rule.HasMutate() {
		checker = mutate.NewMutateFactory(rule.Mutation, client, username)
		if path, err := checker.Validate(context.TODO()); err != nil {
			return "", fmt.Errorf("path: spec.rules[%d].mutate.%s.: %v", idx, path, err)
		}
	}

	// Validate
	if rule.HasValidate() {
		checker = validate.NewValidateFactory(&rule.Validation)
		if path, err := checker.Validate(context.TODO()); err != nil {
			return "", fmt.Errorf("path: spec.rules[%d].validate.%s.: %v", idx, path, err)
		}

		// In case generateValidatingAdmissionPolicy flag is set to true, check the required permissions.
		if toggle.FromContext(context.TODO()).GenerateValidatingAdmissionPolicy() {
			authCheck := authChecker.NewSelfChecker(client.GetKubeClient().AuthorizationV1().SelfSubjectAccessReviews())
			// check if the controller has the required permissions to generate validating admission policies.
			if !validatingadmissionpolicy.HasValidatingAdmissionPolicyPermission(authCheck) {
				return "insufficient permissions to generate ValidatingAdmissionPolicies", nil
			}

			// check if the controller has the required permissions to generate validating admission policy bindings.
			if !validatingadmissionpolicy.HasValidatingAdmissionPolicyBindingPermission(authCheck) {
				return "insufficient permissions to generate ValidatingAdmissionPolicyBindings", nil
			}
		}
	}

	// Generate
	if rule.HasGenerate() {
		// TODO: this check is there to support offline validations
		// generate uses selfSubjectReviews to verify actions
		// this need to modified to use different implementation for online and offline mode
		if mock {
			checker = generate.NewFakeGenerate(rule.Generation)
			if path, err := checker.Validate(context.TODO()); err != nil {
				return "", fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		} else {
			checker = generate.NewGenerateFactory(client, rule.Generation, username, logging.GlobalLogger())
			if path, err := checker.Validate(context.TODO()); err != nil {
				return "", fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		}

		if slices.Contains(rule.MatchResources.Kinds, rule.Generation.Kind) {
			return "", fmt.Errorf("generation kind and match resource kind should not be the same")
		}
	}

	return "", nil
}
