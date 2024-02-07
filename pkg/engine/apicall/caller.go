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
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Caller interface {
	Execute(context.Context, *kyvernov1.APICall) ([]byte, error)
}

type caller struct {
	logger logr.Logger
	name   string
	client ClientInterface
	config APICallConfiguration
}

func NewCaller(
	logger logr.Logger,
	name string,
	client ClientInterface,
	config APICallConfiguration,
) *caller {
	return &caller{
		logger: logger,
		name:   name,
		client: client,
		config: config,
	}
}

func (a *caller) Execute(ctx context.Context, call *kyvernov1.APICall) ([]byte, error) {
	if call.URLPath != "" {
		return a.executeK8sAPICall(ctx, call.URLPath, call.Method, call.Data)
	}
	return a.executeServiceCall(ctx, call)
}

func (a *caller) executeK8sAPICall(ctx context.Context, path string, method kyvernov1.Method, data []kyvernov1.RequestData) ([]byte, error) {
	requestData, err := a.buildRequestData(data)
	if err != nil {
		return nil, err
	}
	jsonData, err := a.client.RawAbsPath(ctx, path, string(method), requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to %v resource with raw url\n: %s: %v", method, path, err)
	}
	a.logger.V(4).Info("executed APICall", "name", a.name, "path", path, "method", method, "len", len(jsonData))
	return jsonData, nil
}

func (a *caller) executeServiceCall(ctx context.Context, apiCall *kyvernov1.APICall) ([]byte, error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.name)
	}
	client, err := a.buildHTTPClient(apiCall.Service)
	if err != nil {
		return nil, err
	}
	req, err := a.buildHTTPRequest(ctx, apiCall)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for APICall %s: %w", a.name, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request for APICall %s: %w", a.name, err)
	}
	defer resp.Body.Close()
	var w http.ResponseWriter
	if a.config.maxAPICallResponseLength != 0 {
		resp.Body = http.MaxBytesReader(w, resp.Body, a.config.maxAPICallResponseLength)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := io.ReadAll(resp.Body)
		if err == nil {
			return nil, fmt.Errorf("HTTP %s: %s", resp.Status, string(b))
		}
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			return nil, fmt.Errorf("response length must be less than max allowed response length of %d.", a.config.maxAPICallResponseLength)
		} else {
			return nil, fmt.Errorf("failed to read data from APICall %s: %w", a.name, err)
		}
	}
	a.logger.Info("executed service APICall", "name", a.name, "len", len(body))
	return body, nil
}

func (a *caller) buildRequestData(data []kyvernov1.RequestData) (io.Reader, error) {
	dataMap := make(map[string]interface{})
	for _, d := range data {
		dataMap[d.Key] = d.Value
	}
	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(dataMap); err != nil {
		return nil, fmt.Errorf("failed to encode HTTP POST data %v for APICall %s: %w", dataMap, a.name, err)
	}
	return buffer, nil
}

func (a *caller) buildHTTPClient(service *kyvernov1.ServiceCall) (*http.Client, error) {
	if service == nil || service.CABundle == "" {
		return http.DefaultClient, nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(service.CABundle)); !ok {
		return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall %s", a.name)
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		},
	}
	return &http.Client{
		Transport: tracing.Transport(transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan)),
	}, nil
}

func (a *caller) buildHTTPRequest(ctx context.Context, apiCall *kyvernov1.APICall) (req *http.Request, err error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service")
	}
	token := a.getToken()
	defer func() {
		if token != "" && req != nil {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}()
	if apiCall.Method == "GET" {
		req, err = http.NewRequestWithContext(ctx, "GET", apiCall.Service.URL, nil)
		return
	}
	if apiCall.Method == "POST" {
		data, dataErr := a.buildRequestData(apiCall.Data)
		if dataErr != nil {
			return nil, dataErr
		}
		req, err = http.NewRequest("POST", apiCall.Service.URL, data)
		return
	}
	return nil, fmt.Errorf("invalid request type %s for APICall %s", apiCall.Method, a.name)
}

func (a *caller) getToken() string {
	fileName := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	b, err := os.ReadFile(fileName)
	if err != nil {
		a.logger.Info("failed to read service account token", "path", fileName)
		return ""
	}
	return string(b)
}
