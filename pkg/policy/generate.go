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
)

func (pc *policyController) handleGenerate(policyKey string, policy kyvernov1.PolicyInterface) error {
	logger := pc.log.WithName("handleGenerate").WithName(policyKey)
	logger.Info("update URs on policy event")

	if err := pc.syncDataPolicyChanges(policy, false); err != nil {
		logger.Error(err, "failed to create UR on policy event")
		return err
	}

	if !policy.GetSpec().IsGenerateExisting() {
		return nil
	}

	logger.V(4).Info("reconcile policy with generateExisting enabled")
	if err := pc.handleGenerateForExisting(policy); err != nil {
		logger.Error(err, "failed to create UR for generateExisting")
		return err
	}
	return nil
}

func (pc *policyController) handleGenerateForExisting(policy kyvernov1.PolicyInterface) error {
	var errors []error
	for _, rule := range policy.GetSpec().Rules {
		ruleType := kyvernov1beta1.Generate
		triggers := generateTriggers(pc.client, rule, pc.log)
		for _, trigger := range triggers {
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

			pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
				"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
		}
	}
	return multierr.Combine(errors...)
}

func (pc *policyController) createURForDownstreamDeletion(policy kyvernov1.PolicyInterface) error {
	var errs []error
	rules := autogen.ComputeRules(policy)
	for _, r := range rules {
		generateType, sync := r.GetGenerateTypeAndSync()
		if sync && (generateType == kyvernov1.Data) {
			if err := pc.syncDataPolicyChanges(policy, true); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return multierr.Combine(errs...)
}

func (pc *policyController) syncDataPolicyChanges(policy kyvernov1.PolicyInterface, deleteDownstream bool) error {
	var errorList []error
	for _, rule := range policy.GetSpec().Rules {
		generate := rule.Generation
		if !generate.Synchronize {
			continue
		}
		if generate.GetData() == nil {
			continue
		}
		if err := pc.syncDataRulechanges(policy, rule, deleteDownstream); err != nil {
			errorList = append(errorList, err)
		}
	}
	return multierr.Combine(errorList...)
}

func (pc *policyController) syncDataRulechanges(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, deleteDownstream bool) error {
	labels := map[string]string{
		common.GeneratePolicyLabel:          policy.GetName(),
		common.GeneratePolicyNamespaceLabel: policy.GetNamespace(),
		common.GenerateRuleLabel:            rule.Name,
		kyvernov1.LabelAppManagedBy:         kyvernov1.ValueKyvernoApp,
	}

	downstreams, err := generateutils.FindDownstream(pc.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), labels)
	if err != nil {
		return err
	}

	pc.log.V(4).Info("sync data rule changes to downstream targets")
	var errorList []error
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
