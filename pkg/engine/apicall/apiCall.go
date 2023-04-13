package apicall

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type apiCall struct {
	logger  logr.Logger
	entry   kyvernov1.ContextEntry
	jsonCtx enginecontext.Interface
	client  dclient.Interface
}

func New(
	log logr.Logger,
	entry kyvernov1.ContextEntry,
	jsonCtx enginecontext.Interface,
	client dclient.Interface,
) (*apiCall, error) {
	if entry.APICall == nil {
		return nil, fmt.Errorf("missing APICall in context entry %v", entry)
	}
	return &apiCall{
		entry:   entry,
		jsonCtx: jsonCtx,
		client:  client,
		logger:  log,
	}, nil
}

func (a *apiCall) Execute(ctx context.Context) ([]byte, error) {
	call, err := variables.SubstituteAllInType(a.logger, a.jsonCtx, a.entry.APICall)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", a.entry.Name, a.entry.APICall.URLPath, err)
	}

	data, err := a.execute(ctx, call)
	if err != nil {
		return nil, err
	}

	result, err := a.transformAndStore(data)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (a *apiCall) execute(ctx context.Context, call *kyvernov1.APICall) ([]byte, error) {
	if call.URLPath != "" {
		return a.executeK8sAPICall(ctx, call.URLPath)
	}

	return a.executeServiceCall(ctx, call.Service)
}

func (a *apiCall) executeK8sAPICall(ctx context.Context, path string) ([]byte, error) {
	jsonData, err := a.client.RawAbsPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource with raw url\n: %s: %v", path, err)
	}

	a.logger.V(4).Info("executed APICall", "name", a.entry.Name, "len", len(jsonData))
	return jsonData, nil
}

func (a *apiCall) executeServiceCall(ctx context.Context, service *kyvernov1.ServiceCall) ([]byte, error) {
	if service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.entry.Name)
	}

	client, err := a.buildHTTPClient(service)
	if err != nil {
		return nil, err
	}

	req, err := a.buildHTTPRequest(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for APICall %s: %w", a.entry.Name, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request for APICall %s: %w", a.entry.Name, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := io.ReadAll(resp.Body)
		if err == nil {
			return nil, fmt.Errorf("HTTP %s: %s", resp.Status, string(b))
		}

		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from APICall %s: %w", a.entry.Name, err)
	}

	a.logger.Info("executed service APICall", "name", a.entry.Name, "len", len(body))
	return body, nil
}

func (a *apiCall) buildHTTPRequest(ctx context.Context, service *kyvernov1.ServiceCall) (req *http.Request, err error) {
	token := a.getToken()
	defer func() {
		if token != "" && req != nil {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}()

	if service.Method == "GET" {
		req, err = http.NewRequestWithContext(ctx, "GET", service.URL, nil)
		return
	}

	if service.Method == "POST" {
		data, dataErr := a.buildPostData(service.Data)
		if dataErr != nil {
			return nil, dataErr
		}

		req, err = http.NewRequest("POST", service.URL, data)
		return
	}

	return nil, fmt.Errorf("invalid request type %s for APICall %s", service.Method, a.entry.Name)
}

func (a *apiCall) getToken() string {
	fileName := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	b, err := os.ReadFile(fileName)
	if err != nil {
		a.logger.Info("failed to read service account token", "path", fileName)
		return ""
	}

	return string(b)
}

func (a *apiCall) buildHTTPClient(service *kyvernov1.ServiceCall) (*http.Client, error) {
	if service.CABundle == "" {
		return http.DefaultClient, nil
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(service.CABundle)); !ok {
		return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall %s", a.entry.Name)
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}, nil
}

func (a *apiCall) buildPostData(data []kyvernov1.RequestData) (io.Reader, error) {
	dataMap := make(map[string]interface{})
	for _, d := range data {
		dataMap[d.Key] = d.Value
	}

	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(dataMap); err != nil {
		return nil, fmt.Errorf("failed to encode HTTP POST data %v for APICall %s: %w", dataMap, a.entry.Name, err)
	}

	return buffer, nil
}

func (a *apiCall) transformAndStore(jsonData []byte) ([]byte, error) {
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

	results, err := applyJMESPathJSON(path.(string), jsonData)
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

func applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}

	jp, err := jmespath.New(jmesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compile JMESPath: %s, error: %v", jmesPath, err)
	}

	return jp.Search(data)
}
