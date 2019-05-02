package engine

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/violation"
	"github.com/nirmata/kube-policy/webhooks"
)

// TODO:
// When the policy get updates, policy controller will detect the changes and
// try to process the changes on all matched resource. If there is any patch
// returns, we should add the violation to the resource indicating the changes
func ApplyRegex(policy types.Policy) (webhooks.PatchBytes, violation.Violations, error) {
	return nil, nil, nil
}
