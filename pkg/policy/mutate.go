package policy

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	backgroundcommon "github.com/kyverno/kyverno/pkg/background/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *policyController) handleMutate(policyKey string, policy kyvernov1.PolicyInterface) error {
	logger := pc.log.WithName("handleMutate").WithName(policyKey)

	logger.Info("update URs on policy event")
	for _, rule := range policy.GetSpec().Rules {
		var ruleType kyvernov1beta1.RequestType
		if rule.IsMutateExisting() {
			ruleType = kyvernov1beta1.Mutate
			triggers := generateTriggers(pc.client, rule, pc.log)
			for _, trigger := range triggers {
				murs := pc.listMutateURs(policyKey, trigger)
				if murs != nil {
					logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+trigger.GetName())
					continue
				}

				logger.Info("creating new UR for mutate")
				ur := newUR(policy, backgroundcommon.ResourceSpecFromUnstructured(*trigger), rule.Name, ruleType, false)
				skip, err := pc.handleUpdateRequest(ur, trigger, rule, policy)
				if err != nil {
					pc.log.Error(err, "failed to create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
					continue
				}
				if skip {
					continue
				}
				pc.log.V(2).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}
		}
	}
	return nil
}

func (pc *policyController) listMutateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	mutateURs, err := pc.urLister.List(labels.SelectorFromSet(backgroundcommon.MutateLabelsSet(policyKey, trigger)))
	if err != nil {
		pc.log.Error(err, "failed to list update request for mutate policy")
	}
	return mutateURs
}
