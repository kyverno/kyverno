package engine

import (
	"log"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GenerationResponse struct {
	Generator *kubepolicy.Generation
	Namespace string
}

// Generate should be called to process generate rules on the resource
// TODO: extend kubeclient(will change to dynamic client) to create resources
func Generate(policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) []GenerationResponse {
	// configMapGenerator and secretGenerator can be applied only to namespaces
	if gvk.Kind != "Namespace" {
		return nil
	}

	var generateResps []GenerationResponse

	for _, rule := range policy.Spec.Rules {
		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)

		if !ok {
			log.Printf("Rule is not applicable to the request: rule name = %s in policy %s \n", rule.Name, policy.ObjectMeta.Name)
			continue
		}

		generateResps, err := applyRuleGenerator(rawResource, rule.Generation)
		if err != nil {
			log.Printf("Failed to apply rule generator: %v", err)
		} else {
			generateResps = append(generateResps, generateResps...)
		}
	}

	return generateResps
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
// TODO: plan to support all kinds of generator
func applyRuleGenerator(rawResource []byte, generator *kubepolicy.Generation) ([]GenerationResponse, error) {
	var generationResponse []GenerationResponse
	if generator == nil {
		return nil, nil
	}

	namespaceName := ParseNameFromObject(rawResource)
	generationResponse = append(generationResponse, GenerationResponse{generator, namespaceName})
	return generationResponse, nil
}
