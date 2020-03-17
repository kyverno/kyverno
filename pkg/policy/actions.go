package policy

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policy/generate"
	"github.com/nirmata/kyverno/pkg/policy/mutate"
	"github.com/nirmata/kyverno/pkg/policy/validate"
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
func validateActions(idx int, rule kyverno.Rule, client *dclient.Client) error {
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
		checker = validate.NewValidateFactory(rule.Validation)
		if path, err := checker.Validate(); err != nil {
			return fmt.Errorf("path: spec.rules[%d].validate.%s.: %v", idx, path, err)
		}
	}

	// Generate
	if rule.HasGenerate() {
		checker = generate.NewGenerateFactory(client, rule.Generation, log.Log)
		if path, err := checker.Validate(); err != nil {
			return fmt.Errorf("path: spec.rules[%d].generate.%s.: %v", idx, path, err)
		}
	}

	return nil
}
