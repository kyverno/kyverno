package store

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

func ContextLoaderFactory(s *Store, cmResolver engineapi.ConfigmapResolver) engineapi.ContextLoaderFactory {
	if !s.IsLocal() {
		var opts []factories.ContextLoaderFactoryOptions
		if gctx := s.GetGlobalContextStore(); gctx != nil {
			opts = append(opts, factories.WithGlobalContextStore(gctx))
		}
		return factories.DefaultContextLoaderFactory(cmResolver, opts...)
	}
	return func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) engineapi.ContextLoader {
		init := func(jsonContext enginecontext.Interface) error {
			rule := s.GetPolicyRule(policy.GetName(), rule.Name)
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
					if err := jsonContext.AddVariable(key, value[s.GetForeachElement()]); err != nil {
						return err
					}
				}
			}
			return nil
		}
		var opts []factories.ContextLoaderFactoryOptions
		opts = append(opts, factories.WithInitializer(init))
		if gctx := s.GetGlobalContextStore(); gctx != nil {
			opts = append(opts, factories.WithGlobalContextStore(gctx))
		}
		factory := factories.DefaultContextLoaderFactory(cmResolver, opts...)
		return wrapper{
			store: s,
			inner: factory(policy, rule),
		}
	}
}

type wrapper struct {
	store *Store
	inner engineapi.ContextLoader
}

func (w wrapper) Load(
	ctx context.Context,
	jp jmespath.Interface,
	client engineapi.RawClient,
	rclientFactory engineapi.RegistryClientFactory,
	contextEntries []kyvernov1.ContextEntry,
	jsonContext enginecontext.Interface,
) error {
	if !w.store.IsApiCallAllowed() {
		client = nil
	}
	if !w.store.GetRegistryAccess() {
		rclientFactory = nil
	}
	return w.inner.Load(ctx, jp, client, rclientFactory, contextEntries, jsonContext)
}
