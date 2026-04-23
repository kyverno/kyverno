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
		if mocks := s.GetGlobalContextEntries(); len(mocks) > 0 {
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
	mockURLIndex, err := buildAPICallURLIndex(w.store.GetAPICallResponses())
	if err != nil {
		return err
	}
	if len(mockURLIndex) > 0 {
		remaining := make([]kyvernov1.ContextEntry, 0, len(contextEntries))
		for _, entry := range contextEntries {
			if entry.APICall != nil {
				url := ""
				if entry.APICall.Service != nil {
					url = entry.APICall.Service.URL
				} else if entry.APICall.URLPath != "" {
					url = entry.APICall.URLPath
				}
				method := string(entry.APICall.Method)
				if url != "" {
					if body, ok := lookupMockResponse(mockURLIndex, method, url); ok {
						data, err := json.Marshal(body)
						if err != nil {
							return err
						}
						if entry.APICall.JMESPath != "" {
							var raw interface{}
							if err := json.Unmarshal(data, &raw); err != nil {
								return fmt.Errorf("failed to unmarshal mock body for %q: %w", entry.Name, err)
							}
							result, err := jp.Search(entry.APICall.JMESPath, raw)
							if err != nil {
								return fmt.Errorf("failed to apply JMESPath %q for context entry %q: %w", entry.APICall.JMESPath, entry.Name, err)
							}
							data, err = json.Marshal(result)
							if err != nil {
								return fmt.Errorf("failed to marshal JMESPath result for %q: %w", entry.Name, err)
							}
						}
						if err := jsonContext.AddContextEntry(entry.Name, data); err != nil {
							return err
						}
						continue
					}
				}
			}
			remaining = append(remaining, entry)
		}
		contextEntries = remaining
	}
	return w.inner.Load(ctx, jp, client, rclientFactory, contextEntries, jsonContext)
}

// lookupMockResponse searches the index by method:url first, then by url alone.
func lookupMockResponse(index map[string]interface{}, method, url string) (interface{}, bool) {
	if method != "" {
		if body, ok := index[method+":"+url]; ok {
			return body, true
		}
	}
	if body, ok := index[url]; ok {
		return body, true
	}
	return nil, false
}

func buildAPICallURLIndex(mocks []v1alpha1.APICallResponseEntry) (map[string]interface{}, error) {
	if len(mocks) == 0 {
		return nil, nil
	}
	index := make(map[string]interface{}, len(mocks))
	for _, m := range mocks {
		body, err := v1alpha1.RawExtensionToObject(m.Response.Body)
		if err != nil {
			return nil, fmt.Errorf("apiCallResponses url %q: invalid body: %w", m.URL, err)
		}
		key := m.URL
		if m.Method != "" {
			key = m.Method + ":" + m.URL
		}
		index[key] = body
	}
	return index, nil
}
