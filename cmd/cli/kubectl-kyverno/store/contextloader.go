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
			storeRule := s.GetPolicyRule(policy.GetName(), rule.Name)
			if storeRule == nil {
				return nil
			}
			if len(storeRule.Values) > 0 {
				for key, value := range storeRule.Values {
					if err := jsonContext.AddVariable(key, value); err != nil {
						return err
					}
				}
			}
			if len(storeRule.ForEachValues) == 0 {
				return nil
			}
			blockIdx := 0
			if raw, err := jsonContext.Query("foreachBlockIndex"); err == nil {
				if v, ok := raw.(int64); ok {
					blockIdx = int(v)
				}
			}
			if blockIdx < 0 || blockIdx >= len(storeRule.ForEachValues) {
				return nil
			}
			blockValues := storeRule.ForEachValues[blockIdx]
			if len(blockValues) == 0 {
				return nil
			}
			elemIdx := 0
			if raw, err := jsonContext.Query("elementIndex"); err == nil {
				if v, ok := raw.(int64); ok {
					elemIdx = int(v)
				}
			}
			for key, values := range blockValues {
				if elemIdx >= 0 && elemIdx < len(values) {
					if err := jsonContext.AddVariable(key, values[elemIdx]); err != nil {
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
