package store

import (
	"context"
	"encoding/json"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

func ContextLoaderFactory(s *Store, cmResolver engineapi.ConfigmapResolver) engineapi.ContextLoaderFactory {
	if !s.IsLocal() {
		return factories.DefaultContextLoaderFactory(cmResolver)
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
		opts := []factories.ContextLoaderFactoryOptions{factories.WithInitializer(init)}
		if mocks := s.GetMockGlobalContextEntries(); len(mocks) > 0 {
			opts = append(opts, factories.WithGlobalContextStore(NewMockGCtxStore(mocks)))
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
	mockURLIndex, err := buildMockAPICallURLIndex(w.store.GetMockAPICallResponses())
	if err != nil {
		return err
	}
	if len(mockURLIndex) > 0 {
		remaining := make([]kyvernov1.ContextEntry, 0, len(contextEntries))
		for _, entry := range contextEntries {
			if entry.APICall != nil && entry.APICall.Service != nil {
				if body, ok := mockURLIndex[entry.APICall.Service.URL]; ok {
					data, err := json.Marshal(body)
					if err != nil {
						return err
					}
					if err := jsonContext.AddContextEntry(entry.Name, data); err != nil {
						return err
					}
					continue
				}
			}
			remaining = append(remaining, entry)
		}
		contextEntries = remaining
	}
	return w.inner.Load(ctx, jp, client, rclientFactory, contextEntries, jsonContext)
}

func buildMockAPICallURLIndex(mocks []v1alpha1.MockAPICallResponse) (map[string]interface{}, error) {
	if len(mocks) == 0 {
		return nil, nil
	}
	index := make(map[string]interface{}, len(mocks))
	for _, m := range mocks {
		body, err := v1alpha1.RawExtensionToObject(m.Response.Body)
		if err != nil {
			return nil, fmt.Errorf("mockAPICallResponses url %q: invalid body: %w", m.URLPath, err)
		}
		index[m.URLPath] = body
	}
	return index, nil
}
