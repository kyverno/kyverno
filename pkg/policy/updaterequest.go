package policy

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateBatchSize is the maximum number of RuleContext entries per UpdateRequest.
// Large clusters with generateExisting/synchronize can produce thousands of triggers,
// causing a single UR to exceed the etcd request size limit.
const generateBatchSize = 100

// submitUR creates an UpdateRequest and sets its status to Pending.
func (pc *policyController) submitUR(ctx context.Context, ur *kyvernov2.UpdateRequest) error {
	created, err := pc.urGenerator.Generate(ctx, pc.kyvernoClient, ur, pc.log)
	if err != nil {
		return err
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(ctx, updated, metav1.UpdateOptions{})
		return err
	}
	return nil
}

func newGenerateUR(policy engineapi.GenericPolicy) *kyvernov2.UpdateRequest {
	ur := newUrMeta()
	if gpol := policy.AsGeneratingPolicy(); gpol != nil {
		ur.Labels = common.GenerateLabelsSet(gpol.GetName())
		ur.Spec = kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: gpol.GetName(),
		}
	} else if ngpol := policy.AsNamespacedGeneratingPolicy(); ngpol != nil {
		policyKey := ngpol.GetNamespace() + "/" + ngpol.GetName()
		ur.Labels = common.GenerateLabelsSet(policyKey)
		ur.Spec = kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policyKey,
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

func addRuleContext(ur *kyvernov2.UpdateRequest, ruleName string, trigger kyverno.ResourceSpec, deleteDownstream, synchronize, cacheRestore bool) {
	ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{
		Rule: ruleName,
		Trigger: kyverno.ResourceSpec{
			Kind:       trigger.GetKind(),
			Namespace:  trigger.GetNamespace(),
			Name:       trigger.GetName(),
			APIVersion: trigger.GetAPIVersion(),
			UID:        trigger.GetUID(),
		},
		DeleteDownstream: deleteDownstream,
		Synchronize:      synchronize,
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
		batch := cloneURWithoutRuleContext(ur)
		batch.Spec.RuleContext = make([]kyvernov2.RuleContext, end-i)
		copy(batch.Spec.RuleContext, ruleCtxs[i:end])
		batches = append(batches, batch)
	}
	return batches
}

func cloneURWithoutRuleContext(ur *kyvernov2.UpdateRequest) *kyvernov2.UpdateRequest {
	batch := &kyvernov2.UpdateRequest{
		TypeMeta: ur.TypeMeta,
		Spec:     ur.Spec,
	}
	ur.ObjectMeta.DeepCopyInto(&batch.ObjectMeta)
	ur.Spec.Context.DeepCopyInto(&batch.Spec.Context)
	ur.Status.DeepCopyInto(&batch.Status)
	batch.Spec.RuleContext = nil
	return batch
}
