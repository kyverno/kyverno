package policy

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	backgroundcommon "github.com/kyverno/kyverno/pkg/background/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *policyController) handleMutate(policyKey string, policy kyvernov1.PolicyInterface) error {
	logger := pc.log.WithName("handleMutate").WithName(policyKey)
	logger.V(4).Info("update URs on policy event")

	ruleType := kyvernov2.Mutate
	spec := policy.GetSpec()
	policyNew := policy.CreateDeepCopy()
	policyNew.GetSpec().Rules = nil

	for _, rule := range spec.Rules {
		if !rule.HasMutateExisting() {
			continue
		}

		mutateExisting := rule.Mutation.MutateExistingOnPolicyUpdate
		if mutateExisting != nil {
			if !*mutateExisting {
				continue
			}
		} else if !spec.MutateExistingOnPolicyUpdate {
			continue
		}

		policyNew.GetSpec().SetRules([]kyvernov1.Rule{rule})
		triggers := getTriggers(pc.client, rule, policyNew.IsNamespaced(), policyNew.GetNamespace(), pc.log)
		for _, trigger := range triggers {
			murs := pc.listMutateURs(policyKey, trigger)
			if murs != nil {
				logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+trigger.GetName())
				continue
			}

			logger.V(4).Info("creating new UR for mutate")
			ur := newMutateUR(policy, backgroundcommon.ResourceSpecFromUnstructured(*trigger), rule.Name)
			skip, err := pc.handleUpdateRequest(ur, trigger, rule.Name, policyNew)
			if err != nil {
				pc.log.Error(err, "failed to create new UR on policy update", "policy", policyNew.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
				continue
			}
			if skip {
				continue
			}
			pc.log.V(2).Info("successfully created UR on policy update", "policy", policyNew.GetName(), "rule", rule.Name, "rule type", ruleType,
				"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
		}
	}
	return nil
}

func (pc *policyController) listMutateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov2.UpdateRequest {
	mutateURs, err := pc.urLister.List(labels.SelectorFromSet(backgroundcommon.MutateLabelsSet(policyKey, trigger)))
	if err != nil {
		pc.log.Error(err, "failed to list update request for mutate policy")
	}
	return mutateURs
}
