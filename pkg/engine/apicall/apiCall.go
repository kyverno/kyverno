package apicall

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type apiCall struct {
	logger   logr.Logger
	jp       jmespath.Interface
	entry    kyvernov1.ContextEntry
	jsonCtx  enginecontext.Interface
	executor Executor
}

func New(
	logger logr.Logger,
	jp jmespath.Interface,
	entry kyvernov1.ContextEntry,
	jsonCtx enginecontext.Interface,
	client ClientInterface,
	apiCallConfig APICallConfiguration,
) (*apiCall, error) {
	if entry.APICall == nil {
		return nil, fmt.Errorf("missing APICall in context entry %v", entry)
	}

	executor := NewExecutor(logger, entry.Name, client, apiCallConfig)

	return &apiCall{
		logger:   logger,
		jp:       jp,
		entry:    entry,
		jsonCtx:  jsonCtx,
		executor: executor,
	}, nil
}

func (a *apiCall) FetchAndLoad(ctx context.Context) ([]byte, error) {
	data, err := a.Fetch(ctx)
	if err != nil {
		return nil, err
	}

	results, err := a.Store(data)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (a *apiCall) Fetch(ctx context.Context) ([]byte, error) {
	call, err := variables.SubstituteAllInType(a.logger, a.jsonCtx, a.entry.APICall)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", a.entry.Name, a.entry.APICall.URLPath, err)
	}
	data, err := a.Execute(ctx, &call.APICall)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *apiCall) Store(data []byte) ([]byte, error) {
	results, err := a.transformAndStore(data)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (a *apiCall) Execute(ctx context.Context, call *kyvernov1.APICall) ([]byte, error) {
	return a.executor.Execute(ctx, call)
}

func (a *apiCall) transformAndStore(jsonData []byte) ([]byte, error) {
	if jsonData == nil {
		if a.entry.APICall.Default.Raw == nil {
			return jsonData, nil
		}
		jsonData = a.entry.APICall.Default.Raw
		err := a.jsonCtx.AddContextEntry(a.entry.Name, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", a.entry.Name, err)
		}

		return jsonData, nil
	}
	if a.entry.APICall.JMESPath == "" {
		err := a.jsonCtx.AddContextEntry(a.entry.Name, jsonData)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", a.entry.Name, err)
		}

		return jsonData, nil
	}

	path, err := variables.SubstituteAll(a.logger, a.jsonCtx, a.entry.APICall.JMESPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s JMESPath %s: %w", a.entry.Name, a.entry.APICall.JMESPath, err)
	}

	results, err := a.applyJMESPathJSON(path.(string), jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to apply JMESPath %s for context entry %s: %w", path, a.entry.Name, err)
	}

	contextData, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("failed to marshall APICall data for context entry %s: %w", a.entry.Name, err)
	}

	err = a.jsonCtx.AddContextEntry(a.entry.Name, contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to add APICall results for context entry %s: %w", a.entry.Name, err)
	}

	a.logger.V(4).Info("added context data", "name", a.entry.Name, "len", len(contextData))
	return contextData, nil
}

func (a *apiCall) applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}
	return a.jp.Search(jmesPath, data)
}
