package policyengine

import (
	"fmt"
	"log"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	event "github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	policyviolation "github.com/nirmata/kube-policy/pkg/policyviolation"
)

type PolicyEngine interface {
	// ProcessMutation should be called from admission contoller
	// when there is an creation / update of the resource
	// ProcessMutation(policy types.Policy, rawResource []byte) (patchBytes []byte, events []Events, err error)
	ProcessMutation(policy types.Policy, rawResource []byte) ([]mutation.PatchBytes, error)

	// ProcessValidation should be called from admission contoller
	// when there is an creation / update of the resource
	ProcessValidation(policy types.Policy, rawResource []byte)

	// ProcessExisting should be called from policy controller
	// when there is an create / update of the policy
	// we should process the policy on matched resources, generate violations accordingly
	ProcessExisting(policy types.Policy, rawResource []byte) ([]policyviolation.Info, []event.Info, error)
}

type policyEngine struct {
	kubeClient *kubeClient.KubeClient
	logger     *log.Logger
}

func NewPolicyEngine(kubeClient *kubeClient.KubeClient, logger *log.Logger) PolicyEngine {
	return &policyEngine{
		kubeClient: kubeClient,
		logger:     logger,
	}
}

func (p *policyEngine) ProcessExisting(policy types.Policy, rawResource []byte) ([]policyviolation.Info, []event.Info, error) {
	var violations []policyviolation.Info
	var events []event.Info

	patchingSets := mutation.GetPolicyPatchingSets(policy)

	for _, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			p.logger.Printf("Invalid rule detected: #%s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			p.logger.Printf("Rule %s of policy %s is not applicable to the request", rule.Name, policy.Name)
			return nil, nil, err
		}

		violation, eventInfos, err := p.processRuleOnResource(policy.Name, rule, rawResource, patchingSets)
		if err != nil {
			p.logger.Printf("Failed to process rule %s, err: %v\n", rule.Name, err)
			continue
		}
		// } else {
		// 	policyPatches = append(policyPatches, processedPatches...)
		// }
		violations = append(violations, violation)
		events = append(events, eventInfos...)
	}
	return violations, events, nil
}

func (p *policyEngine) processRuleOnResource(policyName string, rule types.PolicyRule, rawResource []byte, patchingSets mutation.PatchingSets) (
	policyviolation.Info, []event.Info, error) {

	var violationInfo policyviolation.Info
	var eventInfos []event.Info

	resourceKind := mutation.ParseKindFromObject(rawResource)
	resourceName := mutation.ParseNameFromObject(rawResource)
	resourceNamespace := mutation.ParseNamespaceFromObject(rawResource)

	rulePatchesProcessed, err := mutation.ProcessPatches(rule.Patches, nil, patchingSets)
	if err != nil {
		return violationInfo, eventInfos, fmt.Errorf("Failed to process patches from rule %s: %v", rule.Name, err)
	}

	if rulePatchesProcessed != nil {
		log.Printf("Rule %s: prepared %d patches", rule.Name, len(rulePatchesProcessed))

		violationInfo = policyviolation.NewViolation(policyName, resourceKind, resourceNamespace+"/"+resourceName, rule.Name)
		// add a violation to queue

		// add an event to policy
		//TODO: event msg
		eventInfos = append(eventInfos, event.NewEvent("Policy", policyName, event.PolicyViolation, event.FResourcePolcy))
		// add an event to resource
		eventInfos = append(eventInfos, event.NewEvent(resourceKind, resourceNamespace+"/"+resourceName, event.PolicyViolation, event.FResourcePolcy))
	}

	return violationInfo, eventInfos, nil
}
