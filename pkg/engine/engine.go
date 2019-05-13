package engine

import (
	"fmt"
	"log"

	kubeClient "github.com/nirmata/kube-policy/kubeclient"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/engine/mutation"
	event "github.com/nirmata/kube-policy/pkg/event"
	violation "github.com/nirmata/kube-policy/pkg/violation"
)

type PolicyEngine interface {
	// Mutate should be called from admission contoller
	// when there is an creation / update of the resource
	// Mutate(policy types.Policy, rawResource []byte) (patchBytes []byte, events []Events, err error)
	Mutate(policy types.Policy, rawResource []byte) ([]mutation.PatchBytes, error)

	// Validate should be called from admission contoller
	// when there is an creation / update of the resource
	Validate(policy types.Policy, rawResource []byte)

	// ProcessExisting should be called from policy controller
	// when there is an create / update of the policy
	// we should process the policy on matched resources, generate violations accordingly
	ProcessExisting(policy types.Policy, rawResource []byte) ([]violation.Info, []event.Info, error)
}

type engine struct {
	kubeClient *kubeClient.KubeClient
	logger     *log.Logger
}

func NewPolicyEngine(kubeClient *kubeClient.KubeClient, logger *log.Logger) PolicyEngine {
	return &engine{
		kubeClient: kubeClient,
		logger:     logger,
	}
}

func (e *engine) ProcessExisting(policy types.Policy, rawResource []byte) ([]violation.Info, []event.Info, error) {
	var violations []violation.Info
	var events []event.Info

	patchingSets := mutation.GetPolicyPatchingSets(policy)

	for _, rule := range policy.Spec.Rules {
		err := rule.Validate()
		if err != nil {
			e.logger.Printf("Invalid rule detected: #%s in policy %s, err: %v\n", rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if ok, err := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); !ok {
			e.logger.Printf("Rule %s of policy %s is not applicable to the request", rule.Name, policy.Name)
			return nil, nil, err
		}

		violation, eventInfos, err := e.processRuleOnResource(policy.Name, rule, rawResource, patchingSets)
		if err != nil {
			e.logger.Printf("Failed to process rule %s, err: %v\n", rule.Name, err)
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

func (e *engine) processRuleOnResource(policyName string, rule types.PolicyRule, rawResource []byte, patchingSets mutation.PatchingSets) (
	violation.Info, []event.Info, error) {

	var violationInfo violation.Info
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

		violationInfo = violation.NewViolation(policyName, resourceKind, resourceNamespace+"/"+resourceName, rule.Name)
		// add a violation to queue

		// add an event to policy
		//TODO: event msg
		eventInfos = append(eventInfos, event.NewEvent("Policy", policyName, event.PolicyViolation, event.FResourcePolcy))
		// add an event to resource
		eventInfos = append(eventInfos, event.NewEvent(resourceKind, resourceNamespace+"/"+resourceName, event.PolicyViolation, event.FResourcePolcy))
	}

	return violationInfo, eventInfos, nil
}
