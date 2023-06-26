package store

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
)

func ContextLoaderFactory(cmResolver engineapi.ConfigmapResolver) engineapi.ContextLoaderFactory {
	if !IsMock() {
		return factories.DefaultContextLoaderFactory(cmResolver)
	}
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.ContextLoader {
		init := func(jsonContext enginecontext.Interface) error {
			rule := GetPolicyRule(policy.GetName(), rule.Name)
			if rule != nil && len(rule.Values) > 0 {
				variables := rule.Values
				for key, value := range variables {
					if err := jsonContext.AddVariable(key, value); err != nil {
						return err
					}
				}
			}

			if rule != nil && len(rule.ForEachValues) > 0 {
				for key, value := range rule.ForEachValues {
					if err := jsonContext.AddVariable(key, value[GetForeachElement()]); err != nil {
						return err
					}
				}
			}

			return nil
		}
		factory := factories.DefaultContextLoaderFactory(cmResolver, factories.WithInitializer(init))
		return factory(policy, rule)
	}
}
