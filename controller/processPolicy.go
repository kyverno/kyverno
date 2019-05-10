package controller

import (
	"encoding/json"
	"fmt"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	eventutils "github.com/nirmata/kube-policy/pkg/event/utils"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	violationutils "github.com/nirmata/kube-policy/pkg/violation/utils"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func (c *policyController) runForPolicy(key string) {
	//policy, err := c.GetPolicy(key)
	policy, err := c.getPolicyByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s, err: %v", key, err))
		return
	}

	if policy == nil {
		c.logger.Printf("Counld not find policy by key %s", key)
		return
	}

	violations, events, err := c.processPolicy(*policy)
	if err != nil {
		// add Error processing policy event
	}

	c.logger.Printf("%v, %v", violations, events)
	// TODO:
	// create violations
	// create events

}

// processPolicy process the policy to all the matched resources
func (c *policyController) processPolicy(policy types.Policy) (
	violations []violationutils.ViolationInfo, events []eventutils.EventInfo, err error) {

	for _, rule := range policy.Spec.Rules {
		resources, err := c.filterResourceByRule(rule)
		if err != nil {
			c.logger.Printf("Failed to filter resources by rule %s, err: %v\n", rule.Name, err)
		}

		for _, resource := range resources {
			rawResource, err := json.Marshal(resource)
			if err != nil {
				c.logger.Printf("Failed to marshal resources map to rule %s, err: %v\n", rule.Name, err)
				continue
			}

			violation, eventInfos, err := c.policyEngine.ProcessExisting(policy, rawResource)
			if err != nil {
				c.logger.Printf("Failed to process rule %s, err: %v\n", rule.Name, err)
				continue
			}

			violations = append(violations, violation...)
			events = append(events, eventInfos...)
		}
	}
	return violations, events, nil
}

func (c *policyController) filterResourceByRule(rule types.PolicyRule) ([]runtime.Object, error) {
	var targetResources []runtime.Object
	// TODO: make this namespace all
	var namespace = "default"
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rule detected: %s, err: %v", rule.Name, err)
	}

	// Get the resource list from kind
	resources, err := c.kubeClient.ListResource(rule.Resource.Kind, namespace)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		// TODO:
		rawResource, err := json.Marshal(resource)
		// objKind := resource.GetObjectKind()
		// codecFactory := serializer.NewCodecFactory(runtime.NewScheme())
		// codecFactory.EncoderForVersion()

		if err != nil {
			c.logger.Printf("failed to marshal object %v", resource)
			continue
		}

		// filter the resource by name and label
		if ok, _ := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); ok {
			targetResources = append(targetResources, resource)
		}
	}
	return targetResources, nil
}

func (c *policyController) getPolicyByKey(key string) (*types.Policy, error) {
	// Create nil Selector to grab all the policies
	selector := labels.NewSelector()
	cachedPolicies, err := c.policyLister.List(selector)
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
