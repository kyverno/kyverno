package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
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
		realGctx := s.GetGlobalContextStore()
		if mocks := s.GetGlobalContextEntries(); len(mocks) > 0 {
			mockStore := NewMockGCtxStore(mocks)
			if realGctx != nil {
				opts = append(opts, factories.WithGlobalContextStore(newDelegatingGCtxStore(mockStore, realGctx)))
			} else {
				opts = append(opts, factories.WithGlobalContextStore(mockStore))
			}
		} else if realGctx != nil {
			opts = append(opts, factories.WithGlobalContextStore(realGctx))
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
	if len(mockURLIndex) == 0 {
		return w.inner.Load(ctx, jp, client, rclientFactory, contextEntries, jsonContext)
	}
	if err := factories.RunContextLoaderInitializers(w.inner, jsonContext); err != nil {
		return err
	}
	remaining := make([]kyvernov1.ContextEntry, 0, len(contextEntries))
	for _, entry := range contextEntries {
		if entry.APICall != nil {
			ac := entry.APICall.DeepCopy()
			subbedAc, err := variables.SubstituteAllInType(logr.Discard(), jsonContext, ac)
			if err != nil {
				return fmt.Errorf("failed to substitute variables in apiCall context entry %q: %w", entry.Name, err)
			}
			url := ""
			if subbedAc.Service != nil {
				url = subbedAc.Service.URL
			} else if subbedAc.URLPath != "" {
				url = subbedAc.URLPath
			}
			method := string(subbedAc.Method)
			if method == "" {
				method = "GET"
			}
			if url != "" {
				if mockVal, ok := lookupMockResponse(mockURLIndex, method, url); ok {
					payload, ok := mockVal.(*apiCallMockPayload)
					if !ok {
						return fmt.Errorf("internal: unexpected mock type %T for context entry %q", mockVal, entry.Name)
					}
					if payload.StatusCode < 200 || payload.StatusCode >= 300 {
						return fmt.Errorf("HTTP %d from apiCall mock for context entry %q (v1 apiCall treats non-2xx as failure)", payload.StatusCode, entry.Name)
					}
					data, err := json.Marshal(payload.Body)
					if err != nil {
						return err
					}
					if subbedAc.JMESPath != "" {
						var raw interface{}
						if err := json.Unmarshal(data, &raw); err != nil {
							return fmt.Errorf("failed to unmarshal mock body for %q: %w", entry.Name, err)
						}
						result, err := jp.Search(subbedAc.JMESPath, raw)
						if err != nil {
							return fmt.Errorf("failed to apply JMESPath %q for context entry %q: %w", subbedAc.JMESPath, entry.Name, err)
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
	return factories.LoadContextLoaderEntriesWithoutInitializers(w.inner, ctx, jp, client, rclientFactory, remaining, jsonContext)
}

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

type apiCallMockPayload struct {
	StatusCode int
	Body       interface{}
}

func buildAPICallURLIndex(mocks []v1alpha1.APICallResponseEntry) (map[string]interface{}, error) {
	if len(mocks) == 0 {
		return nil, nil
	}
	index := make(map[string]interface{}, len(mocks))
	for _, m := range mocks {
		body, err := v1alpha1.RawExtensionToObject(m.Response.Body)
		if err != nil {
			url := strings.TrimSpace(m.URL)
			return nil, fmt.Errorf("apiCallResponses url %q: invalid body: %w", url, err)
		}
		sc := m.Response.StatusCode
		if sc == 0 {
			sc = 200
		}
		url := strings.TrimSpace(m.URL)
		method := strings.ToUpper(strings.TrimSpace(m.Method))
		key := url
		if method != "" {
			key = method + ":" + url
		}
		index[key] = &apiCallMockPayload{StatusCode: sc, Body: body}
	}
	return index, nil
}
