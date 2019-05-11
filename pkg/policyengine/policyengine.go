package policyengine

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/policyviolation"
)

// As the logic to process the policies in stateless, we do not need to define struct and implement behaviors for it
// Instead we expose them as standalone functions passing the logger and the required atrributes
// The each function returns the changes that need to be applied on the resource
// the caller is responsible to apply the changes to the resource

//TODO wrap the generate, mutation & validation functions for the existing resources
func ProcessExisting(policy types.Policy, rawResource []byte) ([]policyviolation.Info, []event.Info, error) {
	return nil, nil, nil
}
