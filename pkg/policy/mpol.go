package policy

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
)

// newCELMutateUR creates a bare CELMutate UpdateRequest for a MutatingPolicy background scan.
// There is no admission request context – the processor will fetch all matching resources itself.
func newCELMutateUR(mpol *policiesv1beta1.MutatingPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.CELMutate,
		Policy: mpol.GetName(),
	}
	return ur
}

// newCELMutateURFromNamespacedPolicy creates a bare CELMutate UpdateRequest for a
// NamespacedMutatingPolicy background scan. The policy key uses namespace/name format.
func newCELMutateURFromNamespacedPolicy(nmpol *policiesv1beta1.NamespacedMutatingPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.CELMutate,
		Policy: nmpol.GetNamespace() + "/" + nmpol.GetName(),
	}
	return ur
}

// createURForMutatingPolicy creates a CELMutate UpdateRequest that causes the background
// controller to apply this policy to all currently matching resources.
func (pc *policyController) createURForMutatingPolicy(mpol *policiesv1beta1.MutatingPolicy) error {
	return pc.submitUR(context.TODO(), newCELMutateUR(mpol))
}

// createURForNamespacedMutatingPolicy creates a CELMutate UpdateRequest for a
// NamespacedMutatingPolicy background scan.
func (pc *policyController) createURForNamespacedMutatingPolicy(nmpol *policiesv1beta1.NamespacedMutatingPolicy) error {
	return pc.submitUR(context.TODO(), newCELMutateURFromNamespacedPolicy(nmpol))
}
