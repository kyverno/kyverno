package engine

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
)

type Policy struct {
	Policy         policiesv1beta1.GeneratingPolicyLike
	Exceptions     []*policiesv1beta1.PolicyException
	CompiledPolicy *compiler.Policy
	// LegacyOwnerConflicts lists the namespaces in which a downstream resource
	// generated before the policy namespace label existed has an unknown owner,
	// because a same-named policy of the other scope can generate there.
	// libs.LegacyOwnerAnyNamespace marks every namespace.
	LegacyOwnerConflicts map[string]bool
}
