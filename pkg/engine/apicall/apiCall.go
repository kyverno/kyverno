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

func (a *apiCall) transformAndStore(rawData []byte) ([]byte, error) {
	if rawData == nil {
		if a.entry.APICall.Default.Raw == nil {
			return rawData, nil
		}
		rawData = a.entry.APICall.Default.Raw
		err := a.jsonCtx.AddContextEntry(a.entry.Name, rawData)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", a.entry.Name, err)
		}

		return rawData, nil
	}

	rawData, err := a.convertRawToJSONRaw(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert response to JSON internally (bug): %w", err)
	}

	if a.entry.APICall.JMESPath == "" {
		err := a.jsonCtx.AddContextEntry(a.entry.Name, rawData)
		if err != nil {
			return nil, fmt.Errorf("failed to add resource data to context entry %s: %w", a.entry.Name, err)
		}

		return rawData, nil
	}

	path, err := variables.SubstituteAll(a.logger, a.jsonCtx, a.entry.APICall.JMESPath)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s JMESPath %s: %w", a.entry.Name, a.entry.APICall.JMESPath, err)
	}

	results, err := a.applyJMESPathJSON(path.(string), rawData)
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

// Converts the response content to JSON so that downstream JSON operations can succeed.
func (a *apiCall) convertRawToJSONRaw(rawData []byte) ([]byte, error) {
	responseType := a.entry.APICall.ResponseType
	if responseType == "" {
		responseType = kyvernov1.JSON
	}

	switch responseType {
	case kyvernov1.JSON:
		return rawData, nil
	case kyvernov1.Text:
		return json.Marshal(string(rawData))
	default:
		return nil, fmt.Errorf("unsupported content type %q (bug)", a.entry.APICall.ResponseType)
	}
}

func (a *apiCall) applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}
	return a.jp.Search(jmesPath, data)
}
