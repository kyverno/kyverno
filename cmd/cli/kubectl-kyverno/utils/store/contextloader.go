package store

import (
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
)

func ContextLoaderFactory(
	apiClient engineapi.RawClient,
	registryClient engineapi.RegistryClientFactory,
	cmResolver engineapi.ConfigmapResolver,
) engineapi.ContextLoaderFactory {
	return func(policyName, ruleName string) engineapi.ContextLoader {
		if IsLocal() {
			contextLoaderFactory := createLocalContextLoaderFactory(
				apiClient,
				registryClient,
				cmResolver,
				policyName,
				ruleName,
			)
			return contextLoaderFactory(policyName, ruleName)
		} else {
			contextLoader := factories.DefaultContextLoaderFactory(
				factories.WithAPIClient(apiClient),
				factories.WithRegistryClientFactory(registryClient),
				factories.WithConfigMapResolver(cmResolver),
			)
			return contextLoader(policyName, ruleName)
		}
	}
}

func createLocalContextLoaderFactory(
	apiClient engineapi.RawClient,
	registryClient engineapi.RegistryClientFactory,
	cmResolver engineapi.ConfigmapResolver,
	policy string,
	rule string,
) engineapi.ContextLoaderFactory {
	var opts []factories.ContextLoaderFactoryOptions
	if IsApiCallAllowed() {
		opts = append(opts, factories.WithAPIClient(apiClient))
	}

	if registryClient != nil {
		opts = append(opts, factories.WithRegistryClientFactory(registryClient))
	} else if GetRegistryAccess() {
		rclient := GetRegistryClient()
		rcf := factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil)
		opts = append(opts, factories.WithRegistryClientFactory(rcf))
	}

	init := func(jsonContext enginecontext.Interface) error {
		rule := GetPolicyRule(policy, rule)
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

	opts = append(opts, factories.WithInitializer(init))

	contextLoader := factories.DefaultContextLoaderFactory(opts...)
	return contextLoader
}
