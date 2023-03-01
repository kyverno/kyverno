package policy

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
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
		if err := pc.createUR(policy, rule, false); err != nil {
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

				ur := newUR(policy, common.ResourceSpecFromUnstructured(*trigger), rule.Name, ruleType, false)
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

func (pc *PolicyController) createURForDownstreamDeletion(policy kyvernov1.PolicyInterface) error {
	var errs []error
	rules := autogen.ComputeRules(policy)
	for _, r := range rules {
		generateType, sync := r.GetGenerateTypeAndSync()
		if sync && (generateType == kyvernov1.Data) {
			if err := pc.createUR(policy, r, true); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return multierr.Combine(errs...)
}

func (pc *PolicyController) createUR(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, deleteDownstream bool) error {
	generate := rule.Generation
	if !generate.Synchronize {
		// no action for non-sync policy/rule
		return nil
	}
	var errorList []error
	if generate.GetData() != nil {
		downstreams, err := generateutils.FindDownstream(pc.client, policy, rule)
		if err != nil {
			return err
		}

		for _, downstream := range downstreams.Items {
			labels := downstream.GetLabels()
			trigger := generateutils.TriggerFromLabels(labels)
			ur := newUR(policy, trigger, rule.Name, kyvernov1beta1.Generate, deleteDownstream)
			created, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
			if err != nil {
				errorList = append(errorList, err)
				continue
			}
			updated := created.DeepCopy()
			updated.Status = newURStatus(downstream)
			_, err = pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
			if err != nil {
				errorList = append(errorList, err)
				continue
			}
		}
	}
	return multierr.Combine(errorList...)
}

// ruleDeletion returns true if any rule is deleted, along with deleted rules
func ruleDeletion(old, new kyvernov1.PolicyInterface) (_ kyvernov1.PolicyInterface, ruleDeleted bool) {
	if !new.GetDeletionTimestamp().IsZero() {
		return nil, false
	}

	newRules := new.GetSpec().Rules
	oldRules := old.GetSpec().Rules
	newRulesMap := make(map[string]bool, len(newRules))
	var deletedRules []kyvernov1.Rule

	for _, r := range newRules {
		newRulesMap[r.Name] = true
	}
	for _, r := range oldRules {
		if exist := newRulesMap[r.Name]; !exist {
			deletedRules = append(deletedRules, r)
			ruleDeleted = true
		}
	}

	return buildPolicyWithDeletedRules(old, deletedRules), ruleDeleted
}

func buildPolicyWithDeletedRules(policy kyvernov1.PolicyInterface, deletedRules []kyvernov1.Rule) kyvernov1.PolicyInterface {
	newPolicy := policy.CreateDeepCopy()
	spec := newPolicy.GetSpec()
	spec.SetRules(deletedRules)
	return newPolicy
}
