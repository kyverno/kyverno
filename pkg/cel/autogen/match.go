package autogen

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	podControllerMatchConditionName        = "autogen-"
	PodControllersMatchConditionExpression = "!(object.kind =='Deployment' || object.kind =='ReplicaSet' || object.kind =='StatefulSet' || object.kind =='DaemonSet') || "
	cronjobMatchConditionName              = "autogen-cronjobs-"
	CronJobMatchConditionExpression        = "!(object.kind =='CronJob') || "
)

func createMatchConstraints(configs sets.Set[string], operations []admissionregistrationv1.OperationType) *admissionregistrationv1.MatchResources {
	apps := sets.New[string]()
	batch := sets.New[string]()
	for config := range configs {
		switch config {
		case "jobs", "cronjobs":
			batch = batch.Insert(config)
		case "daemonsets", "deployments", "statefulsets", "replicasets":
			apps = apps.Insert(config)
		}
	}
	rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, 2)
	if apps.Len() > 0 {
		rules = append(rules, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					Resources:   sets.List(apps),
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
				},
				Operations: operations,
			},
		})
	}
	if batch.Len() > 0 {
		rules = append(rules, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					Resources:   sets.List(batch),
					APIGroups:   []string{"batch"},
					APIVersions: []string{"v1"},
				},
				Operations: operations,
			},
		})
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
