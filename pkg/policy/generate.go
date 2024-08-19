package policy

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/common"
	generateutils "github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/config"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (pc *policyController) handleGenerate(policyKey string, policy kyvernov1.PolicyInterface) error {
	logger := pc.log.WithName("handleGenerate").WithName(policyKey)
	logger.Info("update URs on policy event")

	if err := pc.syncDataPolicyChanges(policy, false); err != nil {
		logger.Error(err, "failed to create UR on policy event")
		return err
	}

	logger.V(4).Info("reconcile policy with generateExisting enabled")
	if err := pc.handleGenerateForExisting(policy); err != nil {
		logger.Error(err, "failed to create UR for generateExisting")
		return err
	}
	return nil
}

func (pc *policyController) syncDataPolicyChanges(policy kyvernov1.PolicyInterface, deleteDownstream bool) error {
	var errs []error
	var err error
	ur := newGenerateUR(policy)
	for _, rule := range policy.GetSpec().Rules {
		generate := rule.Generation
		if !generate.Synchronize {
			continue
		}
		if generate.GetData() != nil {
			if ur, err = pc.buildUrForDataRuleChanges(policy, ur, rule.Name, generate.GeneratePatterns, deleteDownstream, false); err != nil {
				errs = append(errs, err)
			}
		}

		for _, foreach := range generate.ForEachGeneration {
			if foreach.GetData() != nil {
				if ur, err = pc.buildUrForDataRuleChanges(policy, ur, rule.Name, foreach.GeneratePatterns, deleteDownstream, false); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(ur.Spec.RuleContext) == 0 {
		return multierr.Combine(errs...)
	}
	pc.log.V(2).WithName("syncDataPolicyChanges").Info("creating new UR for generate")
	created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
	if err != nil {
		errs = append(errs, err)
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (pc *policyController) handleGenerateForExisting(policy kyvernov1.PolicyInterface) error {
	var errors []error
	var triggers []*unstructured.Unstructured
	policyNew := policy.CreateDeepCopy()
	policyNew.GetSpec().Rules = nil
	ur := newGenerateUR(policy)
	logger := pc.log.WithName("handleGenerateForExisting")
	for _, rule := range policy.GetSpec().Rules {
		if !rule.HasGenerate() {
			continue
		}

		// check if the rule sets the generateExisting field.
		// if not, use the policy level setting
		generateExisting := rule.Generation.GenerateExisting
		if generateExisting != nil {
			if !*generateExisting {
				continue
			}
		} else if !policy.GetSpec().GenerateExisting {
			continue
		}

		triggers = getTriggers(pc.client, rule, policy.IsNamespaced(), policy.GetNamespace(), pc.log)
		policyNew.GetSpec().SetRules([]kyvernov1.Rule{rule})
		for _, trigger := range triggers {
			namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), pc.nsLister, pc.log)
			policyContext, err := common.NewBackgroundContext(pc.log, pc.client, ur.Spec.Context, policy, trigger, pc.configuration, pc.jp, namespaceLabels)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to build policy context for rule %s: %w", rule.Name, err))
				continue
			}

			engineResponse := pc.engine.ApplyBackgroundChecks(context.TODO(), policyContext)
			if len(engineResponse.PolicyResponse.Rules) == 0 {
				continue
			}
			logger.V(4).Info("adding rule context", "rule", rule.Name, "trigger", trigger.GetNamespace()+"/"+trigger.GetName())
			addRuleContext(ur, rule.Name, common.ResourceSpecFromUnstructured(*trigger), false)
		}
	}

	if len(ur.Spec.RuleContext) == 0 {
		return multierr.Combine(errors...)
	}

	logger.V(2).Info("creating new UR for generate")
	created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
	if err != nil {
		errors = append(errors, err)
		return multierr.Combine(errors...)
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			errors = append(errors, err)
			return multierr.Combine(errors...)
		}
		pc.log.V(4).Info("successfully created UR on policy update", "policy", policyNew.GetName())
	}
	return multierr.Combine(errors...)
}

func (pc *policyController) createURForDownstreamDeletion(policy kyvernov1.PolicyInterface) error {
	var errs []error
	var err error
	rules := autogen.ComputeRules(policy, "")
	ur := newGenerateUR(policy)
	for _, r := range rules {
		generate := r.Generation
		if !generate.Synchronize {
			continue
		}

		sync, orphanDownstreamOnPolicyDelete := r.GetSyncAndOrphanDownstream()
		if generate.GetData() != nil {
			if sync && (generate.GetType() == kyvernov1.Data) && !orphanDownstreamOnPolicyDelete {
				if ur, err = pc.buildUrForDataRuleChanges(policy, ur, r.Name, r.Generation.GeneratePatterns, true, true); err != nil {
					errs = append(errs, err)
				}
			}
		}

		for _, foreach := range generate.ForEachGeneration {
			if foreach.GetData() != nil {
				if sync && (foreach.GetType() == kyvernov1.Data) && !orphanDownstreamOnPolicyDelete {
					if ur, err = pc.buildUrForDataRuleChanges(policy, ur, r.Name, foreach.GeneratePatterns, true, true); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
	}

	if len(ur.Spec.RuleContext) == 0 {
		return multierr.Combine(errs...)
	}

	pc.log.V(2).WithName("createURForDownstreamDeletion").Info("creating new UR for generate")
	created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
	if err != nil {
		errs = append(errs, err)
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		updated.Status.GeneratedResources = ur.Status.GeneratedResources
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (pc *policyController) buildUrForDataRuleChanges(policy kyvernov1.PolicyInterface, ur *kyvernov2.UpdateRequest, ruleName string, pattern kyvernov1.GeneratePatterns, deleteDownstream, policyDeletion bool) (*kyvernov2.UpdateRequest, error) {
	labels := map[string]string{
		common.GeneratePolicyLabel:          policy.GetName(),
		common.GeneratePolicyNamespaceLabel: policy.GetNamespace(),
		common.GenerateRuleLabel:            ruleName,
		kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
	}

	downstreams, err := common.FindDownstream(pc.client, pattern.GetAPIVersion(), pattern.GetKind(), labels)
	if err != nil {
		return ur, err
	}

	if len(downstreams.Items) == 0 {
		return ur, nil
	}

	pc.log.V(4).Info("sync data rule changes to downstream targets")
	for _, downstream := range downstreams.Items {
		labels := downstream.GetLabels()
		trigger := generateutils.TriggerFromLabels(labels)
		addRuleContext(ur, ruleName, trigger, deleteDownstream)
		if policyDeletion {
			addGeneratedResources(ur, downstream)
		}
	}

	return ur, nil
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
