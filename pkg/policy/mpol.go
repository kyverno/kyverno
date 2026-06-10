package policy

import (
	"context"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// createURForMutatingPolicy creates a CELMutate UpdateRequest that causes the background
// controller to apply this policy to all currently matching resources.
func (pc *policyController) createURForMutatingPolicy(mpol *policiesv1beta1.MutatingPolicy) error {
	ur := newCELMutateUR(mpol)

	created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
	if err != nil {
		return err
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
