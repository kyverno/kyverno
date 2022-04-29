package policy

import (
	"context"
	"fmt"

	"github.com/gardener/controller-manager-library/pkg/logger"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) updateUR(policyKey string, policy kyverno.PolicyInterface) error {
	logger := pc.log.WithName("updateUR").WithName(policyKey)

	// TODO: add check for genExisting
	if !policy.GetSpec().MutateExistingOnPolicyUpdate {
		logger.V(4).Info("skip policy application on policy event", "policyKey", policyKey, "mutateExiting", policy.GetSpec().MutateExistingOnPolicyUpdate)
		return
	}

	logger.Info("update URs on policy event")

	mutateURs := pc.listMutateURs(policyKey, nil)
	generateURs := pc.listGenerateURs(policyKey, nil)
	updateUR(pc.kyvernoClient, policyKey, append(mutateURs, generateURs...), pc.log.WithName("updateUR"))

	for _, rule := range policy.GetSpec().Rules {
		var ruleType urkyverno.RequestType

		if rule.IsMutateExisting() {
			ruleType = urkyverno.Mutate

			triggers := getTriggers(rule)
			for _, trigger := range triggers {
				trigger := trigger // pin it
				murs := pc.listMutateURs(policyKey, &trigger)

				logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.Namespace+trigger.Name)

				if murs != nil {
					continue
				}

				logger.Info("creating new UR")
				ur := newUR(policy, &trigger, ruleType)
				err := pc.handleUpdateRequest(ur)
				if err != nil {
					pc.log.Error(err, "failed to create new UR policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
					continue
				} else {
					pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
				}
			}
		}
		if rule.IsGenerateExisting() {
			ruleType = urkyverno.Generate
			var errors []error
			triggers := generateTriggers(pc.nsLister, rule, pc.log)
			for _, trigger := range triggers {
				trigger := trigger //pin it
				gurs := pc.listGenerateURs(policyKey, &trigger)

				logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.Namespace+trigger.Name)

				if gurs != nil {
					continue
				}

				logger.Info("creating new UR")
				ur := newUR(policy, &trigger, ruleType)
				// here trigger is the namespace for matched kind policy
				triggerResource, err := common.GetResource(pc.client, ur.Spec, pc.log)
				if err != nil {
					pc.log.WithName(rule.Name).Error(err, "failed to get trigger resource")
				}

				policyContext, _, err := common.NewBackgroundContext(pc.client, ur, policy, triggerResource, pc.configHandler, nil, pc.log)
				if err != nil {
					pc.log.WithName(rule.Name).Error(err, "failed to build policy context for genExisting")
					continue
				}
				engineResponse := engine.ApplyBackgroundChecks(policyContext)

				for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
					if ruleResponse.Status != response.RuleStatusPass {
						pc.log.Error(err, "can not create new UR for genExisting rule on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule.Status", ruleResponse.Status)
						errors = append(errors, err)
						continue
					}
					err := pc.handleUpdateRequest(ur)
					if err != nil {
						pc.log.Error(err, "failed to create new UR policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
							"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))

						errors = append(errors, err)
						continue
					} else {
						pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
							"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
					}
				}
			}
			err := engineutils.CombineErrors(errors)
			return err
		}
	}
	return nil
}

func (pc *PolicyController) handleUpdateRequest(ur *urkyverno.UpdateRequest) error {
	new, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), ur, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	new.Status.State = urkyverno.Pending
	if _, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
		pc.log.Error(err, "failed to set UpdateRequest state to Pending")
		return err
	}
	return err
}

func (pc *PolicyController) listMutateURs(policyKey string, trigger *kyverno.ResourceSpec) []*urkyverno.UpdateRequest {
	selector := createMutateLabels(policyKey, trigger)
	mutateURs, err := pc.urLister.List(labels.SelectorFromSet(selector))
	if err != nil {
		logger.Error(err, "failed to list update request for mutate policy")
	}

	return mutateURs
}

func (pc *PolicyController) listGenerateURs(policyKey string, trigger *kyverno.ResourceSpec) []*urkyverno.UpdateRequest {
	selector := createGenerateLabels(policyKey, trigger)
	generateURs, err := pc.urLister.List(labels.SelectorFromSet(selector))
	if err != nil {
		logger.Error(err, "failed to list update request for generate policy")
	}

	return generateURs
}

func newUR(policy kyverno.PolicyInterface, trigger *kyverno.ResourceSpec, ruleType urkyverno.RequestType) *urkyverno.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.IsNamespaced() {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	var label labels.Set
	if ruleType == urkyverno.Mutate {
		label = createMutateLabels(policyNameNamespaceKey, trigger)
	} else {
		label = createGenerateLabels(policyNameNamespaceKey, trigger)
	}

	return &urkyverno.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace,
			Labels:       label,
		},
		Spec: urkyverno.UpdateRequestSpec{
			Type:   ruleType,
			Policy: policyNameNamespaceKey,
			Resource: kyverno.ResourceSpec{
				Kind:       trigger.Kind,
				Namespace:  trigger.Namespace,
				Name:       trigger.Name,
				APIVersion: trigger.APIVersion,
			},
		},
	}
}

func createMutateLabels(policyKey string, trigger *kyverno.ResourceSpec) labels.Set {
	var selector labels.Set
	if trigger == nil {
		selector = labels.Set(map[string]string{
			urkyverno.URMutatePolicyLabel: policyKey,
		})
	} else {
		selector = labels.Set(map[string]string{
			urkyverno.URMutatePolicyLabel:      policyKey,
			urkyverno.URMutateTriggerNameLabel: trigger.Name,
			urkyverno.URMutateTriggerNSLabel:   trigger.Namespace,
			urkyverno.URMutatetriggerKindLabel: trigger.Kind,
		})

		if trigger.APIVersion != "" {
			selector[urkyverno.URMutatetriggerAPIVersionLabel] = trigger.APIVersion
		}
	}

	return selector
}

func createGenerateLabels(policyKey string, trigger *kyverno.ResourceSpec) labels.Set {
	var selector labels.Set
	if trigger == nil {
		selector = labels.Set(map[string]string{
			urkyverno.URGeneratePolicyLabel: policyKey,
		})
	} else {
		selector = labels.Set(map[string]string{
			urkyverno.URGeneratePolicyLabel:          policyKey,
			"generate.kyverno.io/resource-name":      trigger.Name,
			"generate.kyverno.io/resource-kind":      trigger.Kind,
			"generate.kyverno.io/resource-namespace": trigger.Namespace,
		})
	}

	return selector
}
