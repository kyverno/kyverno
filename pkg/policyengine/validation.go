package policyengine

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/policyviolation"
)

// Validation should be called to process validation rules on the resource
//TODO: validate should return a bool to specify if the validation was succesful or not
func Validation(policy types.Policy, rawResource []byte) (bool, []policyviolation.Info, []event.Info) {
	return true, nil, nil
}
