package policy

import (
	"context"
	"fmt"

	"github.com/gardener/controller-manager-library/pkg/logger"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (pc *PolicyController) updateUR(policyKey string, policy kyverno.PolicyInterface) error {
	logger := pc.log.WithName("updateUR").WithName(policyKey)

	if !policy.GetSpec().MutateExistingOnPolicyUpdate && !policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
		logger.V(4).Info("skip policy application on policy event", "policyKey", policyKey, "mutateExiting", policy.GetSpec().MutateExistingOnPolicyUpdate, "generateExisting", policy.GetSpec().IsGenerateExistingOnPolicyUpdate())
		return nil
	}

	logger.Info("update URs on policy event")

	var errors []error
	mutateURs := pc.listMutateURs(policyKey, nil)
	generateURs := pc.listGenerateURs(policyKey, nil)
	updateUR(pc.kyvernoClient, pc.urLister.UpdateRequests(config.KyvernoNamespace), policyKey, append(mutateURs, generateURs...), pc.log.WithName("updateUR"))

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
				ur := newUR(policy, trigger, ruleType)
				skip, err := pc.handleUpdateRequest(ur, trigger, rule, policy)
				if err != nil {
					pc.log.Error(err, "failed to create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
					continue
				}
				if skip {
					continue
				}
				pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}
		}
		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			ruleType = kyvernov1beta1.Generate
			triggers := generateTriggers(pc.client, rule, pc.log)
			for _, trigger := range triggers {
				gurs := pc.listGenerateURs(policyKey, trigger)

				if gurs != nil {
					logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+"/"+trigger.GetName())
					continue
				}

				ur := newUR(policy, trigger, ruleType)
				skip, err := pc.handleUpdateRequest(ur, trigger, rule, policy)
				if err != nil {
					pc.log.Error(err, "failed to create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
					errors = append(errors, err)
					continue
				}

				if skip {
					continue
				}

				pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}
			err := engineutils.CombineErrors(errors)
			return err
		}
	}
	return nil
}

func (pc *PolicyController) handleUpdateRequest(ur *kyvernov1beta1.UpdateRequest, triggerResource *unstructured.Unstructured, rule kyverno.Rule, policy kyverno.PolicyInterface) (skip bool, err error) {
	policyContext, _, err := common.NewBackgroundContext(pc.client, ur, policy, triggerResource, pc.configHandler, nil, pc.log)
	if err != nil {
		return false, errors.Wrapf(err, "failed to build policy context for rule %s", rule.Name)
	}

	engineResponse := engine.ApplyBackgroundChecks(policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		return true, nil
	}

	for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
		if ruleResponse.Status != response.RuleStatusPass {
			pc.log.Error(err, "can not create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule.Status", ruleResponse.Status)
			continue
		}

		pc.log.Info("creating new UR for generate")
		_, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), ur, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
	}
	return false, err
}

func (pc *PolicyController) listMutateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	mutateURs, err := pc.urLister.List(labels.SelectorFromSet(common.MutateLabelsSet(policyKey, trigger)))
	if err != nil {
		logger.Error(err, "failed to list update request for mutate policy")
	}
	return mutateURs
}

func (pc *PolicyController) listGenerateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	generateURs, err := pc.urLister.List(labels.SelectorFromSet(common.GenerateLabelsSet(policyKey, trigger)))
	if err != nil {
		logger.Error(err, "failed to list update request for generate policy")
	}
	return generateURs
}

func newUR(policy kyverno.PolicyInterface, trigger *unstructured.Unstructured, ruleType kyvernov1beta1.RequestType) *kyvernov1beta1.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	var label labels.Set
	if ruleType == kyvernov1beta1.Mutate {
		label = common.MutateLabelsSet(policyNameNamespaceKey, trigger)
	} else {
		label = common.GenerateLabelsSet(policyNameNamespaceKey, trigger)
	}

	return &kyvernov1beta1.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace,
			Labels:       label,
		},
		Spec: kyvernov1beta1.UpdateRequestSpec{
			Type:   ruleType,
			Policy: policyNameNamespaceKey,
			Resource: kyverno.ResourceSpec{
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				APIVersion: trigger.GetAPIVersion(),
			},
		},
	}
}
