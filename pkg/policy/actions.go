package policy

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/policy/mutate"
	"github.com/kyverno/kyverno/pkg/policy/validate"
	"golang.org/x/exp/slices"
)

// Validation provides methods to validate a rule
type Validation interface {
	Validate() (string, error)
}

// validateAction performs validation on the rule actions
// - Mutate
// - Validation
// - Generate
func validateActions(idx int, rule *kyvernov1.Rule, client dclient.Interface, mock bool) error {
	if rule == nil {
		return nil
	}

	var checker Validation
	fmt.Println("======4")
	// Mutate
	if rule.HasMutate() {
		checker = mutate.NewMutateFactory(rule.Mutation)
		if path, err := checker.Validate(); err != nil {
			return fmt.Errorf("path: spec.rules[%d].mutate.%s.: %v", idx, path, err)
		}
	}

	// Validate
	if rule.HasValidate() {
		checker = validate.NewValidateFactory(&rule.Validation)
		if path, err := checker.Validate(); err != nil {
			return fmt.Errorf("path: spec.rules[%d].validate.%s.: %v", idx, path, err)
		}
	}

	fmt.Println("======5")
	// Generate
	if rule.HasGenerate() {
		fmt.Println("======6")
		// TODO: this check is there to support offline validations
		// generate uses selfSubjectReviews to verify actions
		// this need to modified to use different implementation for online and offline mode
		if mock {
			fmt.Println("======7")
			checker = generate.NewFakeGenerate(rule.Generation)
			if path, err := checker.Validate(); err != nil {
				return fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		} else {
			fmt.Println("======8")
			checker = generate.NewGenerateFactory(client, rule.Generation, logging.GlobalLogger())
			if path, err := checker.Validate(); err != nil {
				return fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		}

		if slices.Contains(rule.MatchResources.Kinds, rule.Generation.Kind) {
			return fmt.Errorf("generation kind and match resource kind should not be the same")
		}
	}

	return nil
}
