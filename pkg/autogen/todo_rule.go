package autogen

import (
	"encoding/json"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

// GenerateRules generates rule for podControllers based on scenario A and C
func GenerateRules(spec *kyverno.Spec, controllers string, log logr.Logger) []kyverno.Rule {
	var rules []kyverno.Rule
	for _, rule := range spec.Rules {
		// handle all other controllers other than CronJob
		genRule := generateRuleForControllers(*rule.DeepCopy(), stripCronJob(controllers), log)
		if genRule != nil {
			rules = append(rules, convertRule(*genRule, "Pod"))
		}
		// handle CronJob, it appends an additional rule
		genRule = generateCronJobRule(*rule.DeepCopy(), controllers, log)
		if genRule != nil {
			rules = append(rules, convertRule(*genRule, "Cronjob"))
		}
	}
	return rules
}

func convertRule(rule kyvernoRule, kind string) kyverno.Rule {
	// TODO: marshall, rewrite and unmarshall
	if bytes, err := json.Marshal(rule); err != nil {
		// TODO
	} else {
		bytes = updateGenRuleByte(bytes, kind, rule)
		if err := json.Unmarshal(bytes, &rule); err != nil {
			// TODO
		}
	}
	out := kyverno.Rule{
		Name:         rule.Name,
		VerifyImages: rule.VerifyImages,
	}
	if rule.MatchResources != nil {
		out.MatchResources = *rule.MatchResources
	}
	if rule.ExcludeResources != nil {
		out.ExcludeResources = *rule.ExcludeResources
	}
	if rule.Context != nil {
		out.Context = *rule.Context
	}
	if rule.AnyAllConditions != nil {
		out.AnyAllConditions = *rule.AnyAllConditions
	}
	if rule.Mutation != nil {
		out.Mutation = *rule.Mutation
	}
	if rule.Validation != nil {
		out.Validation = *rule.Validation
	}
	return out
}
