package policy

import (
	"context"
	"fmt"

	"github.com/gardener/controller-manager-library/pkg/logger"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) updateUR(policyKey string, policy kyverno.PolicyInterface) {
	logger := pc.log.WithName("updateUR").WithName(policyKey)
	logger.Info("update URs on policy event")

	mutateURs := pc.listMutateURs(policyKey, nil)
	generateURs := pc.listGenerateURs(policyKey, nil)
	updateUR(pc.kyvernoClient, policy.GetName(), append(mutateURs, generateURs...), pc.log.WithName("updateUR"))

	for _, rule := range policy.GetSpec().Rules {
		var ruleType urkyverno.RequestType
		if rule.IsMutateExisting() {
			ruleType = urkyverno.Mutate
		} else {
			// TODO: assign generate ruleType
			continue
		}

		triggers := getTriggers(rule)
		for _, trigger := range triggers {
			var urs []*urkyverno.UpdateRequest
			if ruleType == urkyverno.Mutate {
				urs = pc.listMutateURs(policyKey, trigger)
			} else {
				urs = pc.listGenerateURs(policyKey, trigger)
			}

			logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.Namespace+trigger.Name)

			if urs != nil {
				continue
			}

			logger.Info("creating new UR")
			ur := newUR(policy, trigger, ruleType)
			new, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), ur, metav1.CreateOptions{})
			if err != nil {
				pc.log.Error(err, "failed to create new UR policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
				continue
			} else {
				pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
			}

			new.Status.State = urkyverno.Pending
			if _, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
				pc.log.Error(err, "failed to set UpdateRequest state to Pending")
			}
		}
	}
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
