package test

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

type Policy struct {
	Rules map[string]Rule
}

type Rule struct {
	Values map[string]interface{}
}

func ContextLoaderFactory(
	cmResolver engineapi.ConfigmapResolver,
	values map[string]Policy,
) engineapi.ContextLoaderFactory {
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.ContextLoader {
		return &mockContextLoader{
			logger:     logging.WithName("MockContextLoaderFactory"),
			policyName: policy.GetName(),
			ruleName:   rule.Name,
			values:     values,
		}
	}
}

type mockContextLoader struct {
	logger       logr.Logger
	policyName   string
	ruleName     string
	values       map[string]Policy
	allowApiCall bool
}

func (l *mockContextLoader) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client dclient.Interface,
	rclient registryclient.Client,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	if l.values != nil {
		policy := l.values[l.policyName]
		if policy.Rules != nil {
			rule := policy.Rules[l.ruleName]
			for key, value := range rule.Values {
				if err := jsonContext.AddVariable(key, value); err != nil {
					return err
				}
			}
		}
	}
	// Context Variable should be loaded after the values loaded from values file
	for _, entry := range contextEntries {
		if entry.ImageRegistry != nil && rclient != nil {
			if err := engineapi.LoadImageData(ctx, jp, rclient, l.logger, entry, jsonContext); err != nil {
				return err
			}
		} else if entry.Variable != nil {
			if err := engineapi.LoadVariable(l.logger, jp, entry, jsonContext); err != nil {
				return err
			}
		} else if entry.APICall != nil && l.allowApiCall {
			if err := engineapi.LoadAPIData(ctx, jp, l.logger, entry, jsonContext, client); err != nil {
				return err
			}
		}
	}
	return nil
}
