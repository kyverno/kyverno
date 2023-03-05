package apicall

import (
	"bytes"
	goctx "context"
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
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
)

type apiCall struct {
	log     logr.Logger
	entry   kyvernov1.ContextEntry
	ctx     goctx.Context
	jsonCtx context.Interface
	client  dclient.Interface
}

func New(ctx goctx.Context, entry kyvernov1.ContextEntry, jsonCtx context.Interface, client dclient.Interface, log logr.Logger) (*apiCall, error) {
	if entry.APICall == nil {
		return nil, fmt.Errorf("missing APICall in context entry %v", entry)
	}

	return &apiCall{
		ctx:     ctx,
		entry:   entry,
		jsonCtx: jsonCtx,
		client:  client,
		log:     log,
	}, nil
}

func (a *apiCall) Execute() ([]byte, error) {
	call, err := variables.SubstituteAllInType(a.log, a.jsonCtx, a.entry.APICall)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", a.entry.Name, a.entry.APICall.URLPath, err)
	}

	data, err := a.execute(call)
	if err != nil {
		return nil, err
	}

	result, err := a.transformAndStore(data)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (a *apiCall) execute(call *kyvernov1.APICall) ([]byte, error) {
	if call.URLPath != "" {
		return a.executeK8sAPICall(call.URLPath)
	}

	return a.executeServiceCall(call.Service)
}

func (a *apiCall) executeK8sAPICall(path string) ([]byte, error) {
	jsonData, err := a.client.RawAbsPath(a.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource with raw url\n: %s: %v", path, err)
	}

	a.log.V(4).Info("executed APICall", "name", a.entry.Name, "len", len(jsonData))
	return jsonData, nil
}

func (a *apiCall) executeServiceCall(service *kyvernov1.ServiceCall) ([]byte, error) {
	if service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.entry.Name)
	}

	client, err := a.buildHTTPClient(service)
	if err != nil {
		return nil, err
	}

	req, err := a.buildHTTPRequest(service)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for APICall %s: %w", a.entry.Name, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request for APICall %s: %w", a.entry.Name, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from APICall %s: %w", a.entry.Name, err)
	}

	a.log.Info("executed service APICall", "name", a.entry.Name, "len", len(body))
	return body, nil
}

func (a *apiCall) buildHTTPRequest(service *kyvernov1.ServiceCall) (req *http.Request, err error) {
	token := a.getToken()
	defer func() {
		if token != "" && req != nil {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}()

	if service.Method == "GET" {
		req, err = http.NewRequest("GET", service.URL, nil)
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
	b, err := os.ReadFile("/var/run/secrets/tokens/api-token")
	if err != nil {
		a.log.Info("failed to read token", "path", "/var/run/secrets/tokens/api-token")
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

	path, err := variables.SubstituteAll(a.log, a.jsonCtx, a.entry.APICall.JMESPath)
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

	a.log.V(4).Info("added context data", "name", a.entry.Name, "len", len(contextData))
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
