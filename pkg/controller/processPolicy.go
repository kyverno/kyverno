package controller

import (
	"encoding/json"
	"fmt"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	engine "github.com/nirmata/kube-policy/pkg/engine"
	event "github.com/nirmata/kube-policy/pkg/event"
	violation "github.com/nirmata/kube-policy/pkg/violation"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func (pc *PolicyController) runForPolicy(key string) {

	policy, err := pc.getPolicyByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s, err: %v", key, err))
		return
	}

	if policy == nil {
		pc.logger.Printf("Counld not find policy by key %s", key)
		return
	}

	violations, events, err := pc.processPolicy(*policy)
	if err != nil {
		// add Error processing policy event
	}

	pc.logger.Printf("%v, %v", violations, events)
	// TODO:
	// create violations
	//	pc.violationBuilder.Add()
	// create events
	//	pc.eventBuilder.Add()

}

// processPolicy process the policy to all the matched resources
func (pc *PolicyController) processPolicy(policy types.Policy) (
	violations []violation.Info, events []event.Info, err error) {

	for _, rule := range policy.Spec.Rules {
		resources, err := pc.filterResourceByRule(rule)
		if err != nil {
			pc.logger.Printf("Failed to filter resources by rule %s, err: %v\n", rule.Name, err)
		}

		for _, resource := range resources {
			rawResource, err := json.Marshal(resource)
			if err != nil {
				pc.logger.Printf("Failed to marshal resources map to rule %s, err: %v\n", rule.Name, err)
				continue
			}

			violation, eventInfos, err := engine.ProcessExisting(policy, rawResource)
			if err != nil {
				pc.logger.Printf("Failed to process rule %s, err: %v\n", rule.Name, err)
				continue
			}

			violations = append(violations, violation...)
			events = append(events, eventInfos...)
		}
	}
	return violations, events, nil
}

func (pc *PolicyController) filterResourceByRule(rule types.Rule) ([]runtime.Object, error) {
	var targetResources []runtime.Object
	// TODO: make this namespace all
	var namespace = "default"
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rule detected: %s, err: %v", rule.Name, err)
	}

	// Get the resource list from kind
	resources, err := pc.kubeClient.ListResource(rule.ResourceDescription.Kind, namespace)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		// TODO:
		//rawResource, err := json.Marshal(resource)
		// objKind := resource.GetObjectKind()
		// codecFactory := serializer.NewCodecFactory(runtime.NewScheme())
		// codecFactory.EncoderForVersion()

		if err != nil {
			pc.logger.Printf("failed to marshal object %v", resource)
			continue
		}

		// filter the resource by name and label
		//if ok, _ := mutation.ResourceMeetsRules(rawResource, rule.ResourceDescription); ok {
		//	targetResources = append(targetResources, resource)
		//}
	}
	return targetResources, nil
}

func (pc *PolicyController) getPolicyByKey(key string) (*types.Policy, error) {
	// Create nil Selector to grab all the policies
	selector := labels.NewSelector()
	cachedPolicies, err := pc.policyLister.List(selector)
	if err != nil {
		return nil, err
	}

	for _, elem := range cachedPolicies {
		if elem.Name == key {
			return elem, nil
		}
	}

	return nil, nil
}
