package autogen

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type autogencontroller string

var (
	PODS     autogencontroller = "pods"
	CRONJOBS autogencontroller = "cronjobs"
)

func generateRuleForControllers(spec *policiesv1alpha1.ValidatingPolicySpec, controllers string, resource autogencontroller) (autogenRule *policiesv1alpha1.AutogenRule, err error) {
	operations := spec.MatchConstraints.ResourceRules[0].Operations
	newSpec := &policiesv1alpha1.ValidatingPolicySpec{}
	// create a resource rule for pod controllers
	newSpec.MatchConstraints = createMatchConstraints(controllers, operations)

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

func generateCronJobRule(spec *policiesv1alpha1.ValidatingPolicySpec, controllers string) (*policiesv1alpha1.AutogenRule, error) {
	return generateRuleForControllers(spec, controllers, CRONJOBS)
}

func generatePodControllerRule(spec *policiesv1alpha1.ValidatingPolicySpec, controllers string) (*policiesv1alpha1.AutogenRule, error) {
	return generateRuleForControllers(spec, controllers, PODS)
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

func convertMatchConditions(conditions []admissionregistrationv1.MatchCondition, resource autogencontroller) (matchConditions []admissionregistrationv1.MatchCondition, err error) {
	var name, expression string
	switch resource {
	case PODS:
		name = podControllerMatchConditionName
		expression = PodControllersMatchConditionExpression
	case CRONJOBS:
		name = cronjobMatchConditionName
		expression = CronJobMatchConditionExpression
	}

	for _, m := range conditions {
		m.Name = name + m.Name
		m.Expression = expression + m.Expression
		matchConditions = append(matchConditions, m)
	}
	return matchConditions, nil
}

var (
	podControllerMatchConditionName        = "autogen-"
	PodControllersMatchConditionExpression = "!(object.kind =='Deployment' || object.kind =='ReplicaSet' || object.kind =='StatefulSet' || object.kind =='DaemonSet') || "
	cronjobMatchConditionName              = "autogen-cronjobs-"
	CronJobMatchConditionExpression        = "!(object.kind =='CronJob') || "
)

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
