package engine

import (
	"encoding/json"
	"fmt"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *policyEngine) Validate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) bool {
	var resource interface{}
	json.Unmarshal(rawResource, &resource)

	allowed := true
	for i, rule := range policy.Spec.Rules {

		// Checks for preconditions
		// TODO: Rework PolicyEngine interface that it receives not a policy, but mutation object for
		// Mutate, validation for Validate and so on. It will allow to bring this checks outside of PolicyEngine
		// to common part as far as they present for all: mutation, validation, generation

		err := rule.Validate()
		if err != nil {
			p.logger.Printf("Rule has invalid structure: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		ok, err := ResourceMeetsRules(rawResource, rule.ResourceDescription, gvk)
		if err != nil {
			p.logger.Printf("Rule has invalid data: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if !ok {
			p.logger.Printf("Rule is not applicable t the request: rule number = %d, rule name = %s in policy %s, err: %v\n", i, rule.Name, policy.ObjectMeta.Name, err)
			continue
		}

		if rule.Validation == nil {
			continue
		}

		if err := traverseAndValidate(resource, rule.Validation.Pattern); err != nil {
			p.logger.Printf("Validation with the rule %s has failed %s: %s\n", rule.Name, err.Error(), *rule.Validation.Message)
			allowed = false
		} else {
			p.logger.Printf("Validation rule %s is successful %s: %s\n", rule.Name, err.Error(), *rule.Validation.Message)
		}
	}

	return allowed
}

func traverseAndValidate(resourcePart, patternPart interface{}) error {
	switch pattern := patternPart.(type) {
	case map[string]interface{}:
		dictionary, ok := resourcePart.(map[string]interface{})

		if !ok {
			return fmt.Errorf("Validating error: expected %T, found %T", patternPart, resourcePart)
		}

		var err error
		for key, value := range pattern {
			err = traverseAndValidate(dictionary[key], value)
		}
		return err

	case []interface{}:
		array, ok := resourcePart.([]interface{})

		if !ok {
			return fmt.Errorf("Validating error: expected %T, found %T", patternPart, resourcePart)
		}

		var err error
		for i, value := range pattern {
			err = traverseAndValidate(array[i], value)
		}
		return err
	case string:
		str := resourcePart.(string)
		if !checkForWildcard(str, pattern) {
			return fmt.Errorf("Value %s has not passed wildcard check %s", str, pattern)
		}
	default:
		return fmt.Errorf("Received unknown type: %T", patternPart)
	}

	return nil
}

func checkForWildcard(value, pattern string) bool {
	return value == pattern
}

func checkForOperator(value int, pattern string) bool {
	return true
}
