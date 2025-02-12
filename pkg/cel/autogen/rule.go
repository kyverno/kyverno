package autogen

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func generateCronJobRule(spec *policiesv1alpha1.ValidatingPolicySpec, controllers string) (*policiesv1alpha1.AutogenRule, error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	// create a resource rule for the cronjob resource
	matchConstraints := createMatchConstraints(controllers, operations)

	// convert match conditions
	matchConditions, err := convertMatchconditions(spec.MatchConditions, "cronjobs", cronjobMatchConditionName, cronJobMatchConditionExpression)
	if err != nil {
		return nil, err
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

	return &policiesv1alpha1.AutogenRule{
		MatchConstraints: matchConstraints,
		MatchConditions:  matchConditions,
		Validations:      validations,
		AuditAnnotation:  auditAnnotations,
		Variables:        variables,
	}, nil
}

func generateRuleForControllers(spec *policiesv1alpha1.ValidatingPolicySpec, controllers string) (*policiesv1alpha1.AutogenRule, error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	// create a resource rule for pod controllers
	matchConstraints := createMatchConstraints(controllers, operations)

	// convert match conditions
	matchConditions, err := convertMatchconditions(spec.MatchConditions, "pods", podControllerMatchConditionName, podControllersMatchConditionExpression)
	if err != nil {
		return nil, err
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

	return &policiesv1alpha1.AutogenRule{
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

func convertMatchconditions(conditions []admissionregistrationv1.MatchCondition, resource, name, expression string) (matchConditions []admissionregistrationv1.MatchCondition, err error) {
	for _, m := range conditions {
		m.Name = name + m.Name
		m.Expression = expression + m.Expression
		matchConditions = append(matchConditions, m)
	}
	if bytes, err := json.Marshal(matchConditions); err != nil {
		return nil, err
	} else {
		bytes = updateFields(bytes, resource)
		if err := json.Unmarshal(bytes, &matchConditions); err != nil {
			return nil, err
		}
	}
	return matchConditions, nil
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

	podControllerMatchConditionName        = "autogen-"
	podControllersMatchConditionExpression = "!(object.Kind =='Deployment' || object.Kind =='ReplicaSet' || object.Kind =='StatefulSet' || object.Kind =='DaemonSet') || "
	cronjobMatchConditionName              = "autogen-cronjobs-"
	cronJobMatchConditionExpression        = "!(object.Kind =='CronJob') || "
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
