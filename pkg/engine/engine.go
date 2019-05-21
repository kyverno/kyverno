package engine

import (
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/violation"
)

// As the logic to process the policies in stateless, we do not need to define struct and implement behaviors for it
// Instead we expose them as standalone functions passing the logger and the required atrributes
// The each function returns the changes that need to be applied on the resource
// the caller is responsible to apply the changes to the resource

func ProcessExisting(policy types.Policy, rawResource []byte) ([]violation.Info, []event.Info, error) {
	var violations []violation.Info
	var events []event.Info

	// TODO:
	// Mutate()
	// Validate()
	return violations, events, nil
}
