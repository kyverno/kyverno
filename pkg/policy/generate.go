package policy

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	generateutils "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/config"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) handleGenerate(policyKey string, policy kyvernov1.PolicyInterface) error {
	var errors []error
	logger := pc.log.WithName("handleGenerate").WithName(policyKey)
	logger.Info("update URs on policy event")

	generateURs := pc.listGenerateURs(policyKey, nil)
	updateUR(pc.kyvernoClient, pc.urLister.UpdateRequests(config.KyvernoNamespace()), policyKey, generateURs, pc.log.WithName("updateUR"))

	for _, rule := range policy.GetSpec().Rules {
		if err := pc.createUR(policy, rule); err != nil {
			logger.Error(err, "failed to create UR on policy event")
		}

		var ruleType kyvernov1beta1.RequestType
		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			ruleType = kyvernov1beta1.Generate
			triggers := generateTriggers(pc.client, rule, pc.log)
			for _, trigger := range triggers {
				gurs := pc.listGenerateURs(policyKey, trigger)
				if gurs != nil {
					logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+"/"+trigger.GetName())
					continue
				}

				ur := newUR(policy, resourceSpecFromUnstructured(trigger), ruleType)
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

				logger.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}
			err := multierr.Combine(errors...)
			return err
		}
	}
	return nil
}

func (pc *PolicyController) listGenerateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	generateURs, err := pc.urLister.List(labels.SelectorFromSet(common.GenerateLabelsSet(policyKey, trigger)))
	if err != nil {
		pc.log.Error(err, "failed to list update request for generate policy")
	}
	return generateURs
}

func (pc *PolicyController) createUR(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) error {
	generate := rule.Generation
	if !generate.Synchronize {
		// no action for non-sync policy/rule
		return nil
	}

	if generate.GetData() != nil {
		downstream, err := pc.client.GetResource(context.TODO(), generate.APIVersion, generate.Kind, generate.Namespace, generate.Name)
		if err != nil {
			return err
		}

		labels := downstream.GetLabels()
		trigger := generateutils.TriggerFromLabels(labels)
		ur := newUR(policy, trigger, kyvernov1beta1.Generate)
		created, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		updated := created.DeepCopy()
		updated.Status.State = kyvernov1beta1.Pending
		_, err = pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
