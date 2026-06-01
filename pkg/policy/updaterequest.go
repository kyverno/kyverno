package policy

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newMutateUR(policy kyvernov1.PolicyInterface, trigger kyvernov1.ResourceSpec, ruleName string) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	ur.Labels = common.MutateLabelsSet(policyKey(policy), trigger)
	ur.Spec = kyvernov2.UpdateRequestSpec{
		Type:   kyvernov2.Mutate,
		Policy: policyKey(policy),
		Rule:   ruleName,
		Resource: kyvernov1.ResourceSpec{
			Kind:       trigger.GetKind(),
			Namespace:  trigger.GetNamespace(),
			Name:       trigger.GetName(),
			APIVersion: trigger.GetAPIVersion(),
			UID:        trigger.GetUID(),
		},
	}
	return ur
}

func newGenerateUR(policy engineapi.GenericPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	if kpol := policy.AsKyvernoPolicy(); kpol != nil {
		ur.Labels = common.GenerateLabelsSet(policyKey(kpol))
		ur.Spec = kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.Generate,
			Policy: policyKey(kpol),
		}
	} else if gpol := policy.AsGeneratingPolicy(); gpol != nil {
		ur.Labels = common.GenerateLabelsSet(gpol.GetName())
		ur.Spec = kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: gpol.GetName(),
		}
	}
	return ur
}

func newUrMeta() *kyvernov2.UpdateRequest {
	return &kyvernov2.UpdateRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kyvernov2.SchemeGroupVersion.String(),
			Kind:       "UpdateRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace(),
		},
	}
}

func addGeneratedResources(ur *kyvernov2.UpdateRequest, downstream unstructured.Unstructured) {
	ur.Status.GeneratedResources = append(ur.Status.GeneratedResources,
		kyvernov1.ResourceSpec{
			APIVersion: downstream.GetAPIVersion(),
			Kind:       downstream.GetKind(),
			Namespace:  downstream.GetNamespace(),
			Name:       downstream.GetName(),
			UID:        downstream.GetUID(),
		},
	)
}

func addRuleContext(ur *kyvernov2.UpdateRequest, ruleName string, trigger kyvernov1.ResourceSpec, deleteDownstream, cacheRestore bool) {
	ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{
		Rule: ruleName,
		Trigger: kyvernov1.ResourceSpec{
			Kind:       trigger.GetKind(),
			Namespace:  trigger.GetNamespace(),
			Name:       trigger.GetName(),
			APIVersion: trigger.GetAPIVersion(),
			UID:        trigger.GetUID(),
		},
		DeleteDownstream: deleteDownstream,
		CacheRestore:     cacheRestore,
	})
}

// splitUR splits a single UpdateRequest into multiple URs each containing at most
// batchSize RuleContext entries. This prevents etcd "request is too large" errors
// when policies match thousands of existing resources.
func splitUR(ur *kyvernov2.UpdateRequest, batchSize int) []*kyvernov2.UpdateRequest {
	if batchSize <= 0 {
		batchSize = 1
	}
	if len(ur.Spec.RuleContext) <= batchSize {
		return []*kyvernov2.UpdateRequest{ur}
	}
	var batches []*kyvernov2.UpdateRequest
	ruleCtxs := ur.Spec.RuleContext
	for i := 0; i < len(ruleCtxs); i += batchSize {
		end := i + batchSize
		if end > len(ruleCtxs) {
			end = len(ruleCtxs)
		}
		batch := ur.DeepCopy()
		batch.Spec.RuleContext = make([]kyvernov2.RuleContext, end-i)
		copy(batch.Spec.RuleContext, ruleCtxs[i:end])
		batches = append(batches, batch)
	}
	return batches
}
