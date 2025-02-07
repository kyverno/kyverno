package autogen

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func generateCronJobRule(spec *kyvernov2alpha1.ValidatingPolicySpec, controllers string) (*kyvernov2alpha1.AutogenRule, error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	// create a resource rule for the cronjob resource
	matchConstraints := createMatchConstraints(controllers, operations)

	// convert match conditions
	matchConditions := spec.MatchConditions
	if bytes, err := json.Marshal(matchConditions); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, controllers)
		if err := json.Unmarshal(bytes, &matchConditions); err != nil {
			return nil, err
		}
	}

	// convert validations
	validations := spec.Validations
	for i := range validations {
		if bytes, err := json.Marshal(validations[i]); err != nil {
			return nil, err
		} else {
			bytes = updateFields(bytes, controllers)
			if err := json.Unmarshal(bytes, &validations[i]); err != nil {
				return nil, err
			}
		}
	}

	// convert audit annotations
	auditAnnotations := spec.AuditAnnotations
	if bytes, err := json.Marshal(auditAnnotations); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, controllers)
		if err := json.Unmarshal(bytes, &auditAnnotations); err != nil {
			return nil, err
		}
	}

	// convert variables
	variables := spec.Variables
	if bytes, err := json.Marshal(variables); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, controllers)
		if err := json.Unmarshal(bytes, &variables); err != nil {
			return nil, err
		}
	}

	return &kyvernov2alpha1.AutogenRule{
		MatchConstraints: matchConstraints,
		MatchConditions:  matchConditions,
		Validations:      validations,
		AuditAnnotation:  auditAnnotations,
		Variables:        variables,
	}, nil
}

func generateRuleForControllers(spec *kyvernov2alpha1.ValidatingPolicySpec, controllers string) (*kyvernov2alpha1.AutogenRule, error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	// create a resource rule for pod controllers
	matchConstraints := createMatchConstraints(controllers, operations)

	// convert match conditions
	matchConditions := spec.MatchConditions
	if bytes, err := json.Marshal(matchConditions); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, "pods")
		if err := json.Unmarshal(bytes, &matchConditions); err != nil {
			return nil, err
		}
	}

	// convert validations
	validations := spec.Validations
	if bytes, err := json.Marshal(validations); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, "pods")
		if err := json.Unmarshal(bytes, &validations); err != nil {
			return nil, err
		}
	}

	// convert audit annotations
	auditAnnotations := spec.AuditAnnotations
	if bytes, err := json.Marshal(auditAnnotations); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, "pods")
		if err := json.Unmarshal(bytes, &auditAnnotations); err != nil {
			return nil, err
		}
	}

	// convert variables
	variables := spec.Variables
	if bytes, err := json.Marshal(variables); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, "pods")
		if err := json.Unmarshal(bytes, &variables); err != nil {
			return nil, err
		}
	}

	return &kyvernov2alpha1.AutogenRule{
		MatchConstraints: matchConstraints,
		MatchConditions:  matchConditions,
		Validations:      validations,
		AuditAnnotation:  auditAnnotations,
		Variables:        variables,
	}, nil
}

func createMatchConstraints(controllers string, operations []admissionregistrationv1.OperationType) *admissionregistrationv1.MatchResources {
	resources := strings.Split(controllers, ",")

	var rules []admissionregistrationv1.NamedRuleWithOperations
	for _, resource := range resources {
		if resource == "jobs" || resource == "cronjobs" {
			rules = append(rules, admissionregistrationv1.NamedRuleWithOperations{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources:   []string{resource},
						APIGroups:   []string{"batch"},
						APIVersions: []string{"v1"},
					},
					Operations: operations,
				},
			})
		} else {
			newRule := true
			for i := range rules {
				if slices.Contains(rules[i].APIGroups, "apps") && slices.Contains(rules[i].APIVersions, "v1") {
					rules[i].Resources = append(rules[i].Resources, resource)
					newRule = false
					break
				}
			}

			if newRule {
				rules = append(rules, admissionregistrationv1.NamedRuleWithOperations{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							Resources:   []string{resource},
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
						},
						Operations: operations,
					},
				})
			}
		}
	}

	return &admissionregistrationv1.MatchResources{
		ResourceRules: rules,
	}
}

var (
	podReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("object.spec"), []byte("object.spec.template.spec")},
		{[]byte("oldObject.spec"), []byte("oldObject.spec.template.spec")},
		{[]byte("object.metadata"), []byte("object.spec.template.metadata")},
		{[]byte("oldObject.metadata"), []byte("oldObject.spec.template.metadata")},
	}
	cronJobReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("object.spec"), []byte("object.spec.jobTemplate.spec.template.spec")},
		{[]byte("oldObject.spec"), []byte("oldObject.spec.jobTemplate.spec.template.spec")},
		{[]byte("object.metadata"), []byte("object.spec.jobTemplate.spec.template.metadata")},
		{[]byte("oldObject.metadata"), []byte("oldObject.spec.jobTemplate.spec.template.metadata")},
	}
)

func updateFields(data []byte, resource string) []byte {
	switch resource {
	case "pods":
		for _, replacement := range podReplacementRules {
			data = bytes.ReplaceAll(data, replacement[0], replacement[1])
		}
	case "cronjobs":
		for _, replacement := range cronJobReplacementRules {
			data = bytes.ReplaceAll(data, replacement[0], replacement[1])
		}
	}
	return data
}
