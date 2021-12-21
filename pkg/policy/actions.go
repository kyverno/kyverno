package policy

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/policy/mutate"
	"github.com/kyverno/kyverno/pkg/policy/validate"
	"github.com/kyverno/kyverno/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//Validation provides methods to validate a rule
type Validation interface {
	Validate() (string, error)
}

//validateAction performs validation on the rule actions
// - Mutate
// - Validation
// - Generate
func validateActions(idx int, rule *kyverno.Rule, client *dclient.Client, mock bool) error {
	if rule == nil {
		return nil
	}

	var checker Validation

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

	// Generate
	if rule.HasGenerate() {
		//TODO: this check is there to support offline validations
		// generate uses selfSubjectReviews to verify actions
		// this need to modified to use different implementation for online and offline mode
		if mock {
			checker = generate.NewFakeGenerate(rule.Generation)
			if path, err := checker.Validate(); err != nil {
				return fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		} else {
			checker = generate.NewGenerateFactory(client, rule.Generation, log.Log)
			if path, err := checker.Validate(); err != nil {
				return fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
			}
		}

		if utils.ContainsString(rule.MatchResources.Kinds, rule.Generation.Kind) {
			return fmt.Errorf("generation kind and match resource kind should not be the same.")
		}
	}

	return nil
}
