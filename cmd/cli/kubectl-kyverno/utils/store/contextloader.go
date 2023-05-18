package store

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

func ContextLoaderFactory(
	cmResolver engineapi.ConfigmapResolver,
) engineapi.ContextLoaderFactory {
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.ContextLoader {
		inner := engineapi.DefaultContextLoaderFactory(cmResolver)
		if IsMock() {
			return &mockContextLoader{
				logger:     logging.WithName("MockContextLoaderFactory"),
				policyName: policy.GetName(),
				ruleName:   rule.Name,
			}
		} else {
			return inner(policy, rule)
		}
	}
}

type mockContextLoader struct {
	logger     logr.Logger
	policyName string
	ruleName   string
}

func (l *mockContextLoader) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client dclient.Interface,
	_ registryclient.Client,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	rule := GetPolicyRule(l.policyName, l.ruleName)
	if rule != nil && len(rule.Values) > 0 {
		variables := rule.Values
		for key, value := range variables {
			if err := jsonContext.AddVariable(key, value); err != nil {
				return err
			}
		}
	}
	hasRegistryAccess := GetRegistryAccess()
	// Context Variable should be loaded after the values loaded from values file
	for _, entry := range contextEntries {
		if entry.ImageRegistry != nil && hasRegistryAccess {
			rclient := GetRegistryClient()
			if err := engineapi.LoadImageData(ctx, jp, rclient, l.logger, entry, jsonContext); err != nil {
				return err
			}
		} else if entry.Variable != nil {
			if err := engineapi.LoadVariable(l.logger, jp, entry, jsonContext); err != nil {
				return err
			}
		} else if entry.APICall != nil && IsApiCallAllowed() {
			if err := engineapi.LoadAPIData(ctx, jp, l.logger, entry, jsonContext, client); err != nil {
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
