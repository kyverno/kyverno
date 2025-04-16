package autogen

import (
	"bytes"
	"encoding/json"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type autogencontroller string

var (
	PODS     autogencontroller = "pods"
	CRONJOBS autogencontroller = "cronjobs"
)

func generateRuleForControllers(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string], resource autogencontroller) (autogenRule *policiesv1alpha1.AutogenRule, err error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	newSpec := &policiesv1alpha1.ValidatingPolicySpec{}
	// create a resource rule for pod controllers
	newSpec.MatchConstraints = createMatchConstraints(configs, operations)
	// convert match conditions
	newSpec.MatchConditions, err = convertMatchConditions(spec.MatchConditions, resource)
	if err != nil {
		return nil, err
	}
	newSpec.Validations = spec.Validations
	newSpec.AuditAnnotations = spec.AuditAnnotations
	newSpec.Variables = spec.Variables
	if bytes, err := json.Marshal(newSpec); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, resource)
		if err := json.Unmarshal(bytes, &newSpec); err != nil {
			return nil, err
		}
	}
	return &policiesv1alpha1.AutogenRule{
		MatchConstraints: newSpec.MatchConstraints,
		MatchConditions:  newSpec.MatchConditions,
		Validations:      newSpec.Validations,
		AuditAnnotation:  newSpec.AuditAnnotations,
		Variables:        newSpec.Variables,
	}, nil
}

func generateCronJobRule(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) (*policiesv1alpha1.AutogenRule, error) {
	return generateRuleForControllers(spec, configs, CRONJOBS)
}

func generatePodControllerRule(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) (*policiesv1alpha1.AutogenRule, error) {
	return generateRuleForControllers(spec, configs, PODS)
}

func updateFields(data []byte, resource autogencontroller) []byte {
	// Define the target prefixes based on resource type
	var specPrefix, metadataPrefix []byte
	switch resource {
	case PODS:
		specPrefix = []byte("object.spec.template.spec")
		metadataPrefix = []byte("object.spec.template.metadata")
	case CRONJOBS:
		specPrefix = []byte("object.spec.jobTemplate.spec.template.spec")
		metadataPrefix = []byte("object.spec.jobTemplate.spec.template.metadata")
	}
	// Replace object.spec and oldObject.spec with the correct prefix
	data = bytes.ReplaceAll(data, []byte("object.spec"), specPrefix)
	data = bytes.ReplaceAll(data, []byte("oldObject.spec"), append([]byte("oldObject"), specPrefix[6:]...)) // Adjust for oldObject
	data = bytes.ReplaceAll(data, []byte("object.metadata"), metadataPrefix)
	data = bytes.ReplaceAll(data, []byte("oldObject.metadata"), append([]byte("oldObject"), metadataPrefix[6:]...))
	// Normalize any over-nested paths remove extra .template.spec
	if resource == CRONJOBS {
		data = bytes.ReplaceAll(data, []byte("object.spec.jobTemplate.spec.template.spec.template.spec"), specPrefix)
		data = bytes.ReplaceAll(data, []byte("oldObject.spec.jobTemplate.spec.template.spec.template.spec"), append([]byte("oldObject"), specPrefix[6:]...))
	} else if resource == PODS {
		data = bytes.ReplaceAll(data, []byte("object.spec.template.spec.template.spec"), specPrefix)
		data = bytes.ReplaceAll(data, []byte("oldObject.spec.template.spec.template.spec"), append([]byte("oldObject"), specPrefix[6:]...))
	}
	return data
}
