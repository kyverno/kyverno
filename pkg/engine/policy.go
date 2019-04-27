package engine

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/violation"
	"github.com/nirmata/kube-policy/webhooks"
)

type policyInterface interface {
	ApplySingle(policy types.Policy, resourceRaw []byte) (webhooks.PatchBytes, violation.Violations, error)

	ApplyRegex(policy types.Policy) (webhooks.PatchBytes, violation.Violations, error)
}
