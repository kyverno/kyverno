package policycontroller

import (
	"errors"
	"fmt"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/event"
	"github.com/nirmata/kube-policy/pkg/policyengine"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
	"github.com/nirmata/kube-policy/pkg/policyviolation"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func (pc *PolicyController) runForPolicy(key string) {

	policy, err := pc.getPolicyByKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s, err: %v", key, err))
		return
	}

	if policy == nil {
		pc.logger.Printf("Could not find policy by key %s", key)
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
	violations []policyviolation.Info, events []event.Info, err error) {

	for _, rule := range policy.Spec.Rules {
		resources, err := pc.filterResourceByRule(rule)
		if err != nil {
			pc.logger.Printf("Failed to filter resources by rule %s, err: %v\n", rule.Name, err)
		}

		for _, resource := range resources {
			if err != nil {
				pc.logger.Printf("Failed to marshal resources map to rule %s, err: %v\n", rule.Name, err)
				continue
			}

			violation, eventInfos, err := pc.processExisting(policy, resource)
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

func (pc *PolicyController) filterResourceByRule(rule types.PolicyRule) ([][]byte, error) {
	var targetResources [][]byte
	// TODO: make this namespace all
	var namespace = "default"
	if err := rule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rule detected: %s, err: %v", rule.Name, err)
	}

	// Get the resource list from kind
	resources, err := pc.client.ListResource(rule.Resource.Kind, namespace)
	//	resources, err := pc.kubeClient.ListResource(rule.Resource.Kind, namespace)
	if err != nil {
		return nil, err
	}

	for _, resource := range resources.Items {
		// TODO:
		rawResource, err := resource.MarshalJSON()
		// objKind := resource.GetObjectKind()
		// codecFactory := serializer.NewCodecFactory(runtime.NewScheme())
		// codecFactory.EncoderForVersion()
		if err != nil {
			pc.logger.Printf("failed to marshal object %v", resource)
			continue
		}

		// filter the resource by name and label
		if ok, _ := mutation.IsRuleApplicableToResource(rawResource, rule.Resource); ok {
			targetResources = append(targetResources, rawResource)
		}
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

//TODO wrap the generate, mutation & validation functions for the existing resources
//ProcessExisting processes the policy rule types for the existing resources
func (pc *PolicyController) processExisting(policy types.Policy, rawResource []byte) ([]policyviolation.Info, []event.Info, error) {
	// Generate
	generatedDataList, err := policyengine.Generate(pc.logger, policy, rawResource)
	if err != nil {
		return nil, nil, err
	}
	// apply the generateData using the kubeClient
	err = pc.applyGenerate(generatedDataList)
	if err != nil {
		return nil, nil, err
	}
	// Mutation
	mutationPatches, err := policyengine.Mutation(pc.logger, policy, rawResource)
	if err != nil {
		return nil, nil, err
	}
	// Apply mutationPatches on the rawResource
	err = pc.applyPatches(mutationPatches, rawResource)
	if err != nil {
		return nil, nil, err
	}
	//Validation
	validate, _, _ := policyengine.Validation(policy, rawResource)
	if !validate {
		// validation has errors -> so there will be violations
		// call the violatio builder to apply the violations
	}
	// Generate events

	return nil, nil, nil
}

//TODO: return events and policy violations
func (pc *PolicyController) applyGenerate(generatedDataList []policyengine.GenerateReturnData) error {
	for _, generateData := range generatedDataList {
		switch generateData.ConfigKind {
		case "ConfigMap":
			err := pc.client.GenerateConfigMap(generateData.Generator, generateData.Namespace)
			if err != nil {
				return err
			}
		case "Secret":
			err := pc.client.GenerateSecret(generateData.Generator, generateData.Namespace)
			if err != nil {
				return err
			}
		default:
			return errors.New("Unsuported config kind")
		}
	}
	return nil
}

func (pc *PolicyController) applyPatches([]mutation.PatchBytes, []byte) error {
	return nil
}
