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
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/klog/v2"
)

type APICall struct {
	logger  logr.Logger
	jp      jmespath.Interface
	entry   entry
	jsonCtx enginecontext.Interface
	client  ClientInterface
	config  APICallConfiguration
}

type entry struct {
	Name    string
	APICall *kyvernov1.APICall
}

func parseentry(ent kyvernov1.ContextEntry) entry {
	e := entry{
		Name: ent.Name,
	}
	if ent.APICall != nil {
		e.APICall = ent.APICall
	} else {
		e.APICall = &ent.ResourceCache.APICall.APICall
	}
	return e
}

type APICallConfiguration struct {
	maxAPICallResponseLength int64
	FailedFetchesCounter     metric.Int64Counter
}

func NewAPICallConfiguration(maxLen int64) APICallConfiguration {
	meter := otel.GetMeterProvider().Meter(metrics.MeterName)
	failedFetchesCounter, err := meter.Int64Counter(
		"kyverno_failed_api_calls",
		metric.WithDescription("can be used to track the number of failed api calls"),
	)
	if err != nil {
		logger := klog.Background().V(int(0))
		logger.Error(err, "failed to register metric kyverno_failed_api_calls")
	}

	return APICallConfiguration{
		maxAPICallResponseLength: maxLen,
		FailedFetchesCounter:     failedFetchesCounter,
	}
}

type ClientInterface interface {
	RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error)
}

func New(
	logger logr.Logger,
	jp jmespath.Interface,
	entry kyvernov1.ContextEntry,
	jsonCtx enginecontext.Interface,
	client ClientInterface,
	apiCallConfig APICallConfiguration,
) (*APICall, error) {
	if entry.APICall == nil && (entry.ResourceCache == nil || entry.ResourceCache.APICall == nil) {
		return nil, fmt.Errorf("missing APICall in context entry %v", entry)
	}
	return &APICall{
		logger:  logger,
		jp:      jp,
		entry:   parseentry(entry),
		jsonCtx: jsonCtx,
		client:  client,
		config:  apiCallConfig,
	}, nil
}

func (a *APICall) FetchAndLoad(ctx context.Context) ([]byte, error) {
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

func (a *APICall) Fetch(ctx context.Context) ([]byte, error) {
	call, err := variables.SubstituteAllInType(a.logger, a.jsonCtx, a.entry.APICall)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in context entry %s %s: %v", a.entry.Name, a.entry.APICall.URLPath, err)
	}
	data, err := a.Execute(ctx, call)
	if err != nil {
		metricLabels := []attribute.KeyValue{
			attribute.String("url", string(call.URLPath)),
		}
		a.config.FailedFetchesCounter.Add(ctx, 1, metric.WithAttributes(metricLabels...))
		return nil, err
	}
	return data, nil
}

func (a *APICall) Store(data []byte) ([]byte, error) {
	results, err := a.transformAndStore(data)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (a *APICall) Execute(ctx context.Context, call *kyvernov1.APICall) ([]byte, error) {
	if call.URLPath != "" {
		if a.client == nil {
			return nil, fmt.Errorf("client not found for K8s API Call %s", a.entry.Name)
		}
		return a.executeK8sAPICall(ctx, call.URLPath, call.Method, call.Data)
	}

	return a.executeServiceCall(ctx, call)
}

func (a *APICall) executeK8sAPICall(ctx context.Context, path string, method kyvernov1.Method, data []kyvernov1.RequestData) ([]byte, error) {
	requestData, err := a.buildRequestData(data)
	if err != nil {
		return nil, err
	}

	jsonData, err := a.client.RawAbsPath(ctx, path, string(method), requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to %v resource with raw url\n: %s: %v", method, path, err)
	}

	a.logger.V(4).Info("executed APICall", "name", a.entry.Name, "path", path, "method", method, "len", len(jsonData))
	return jsonData, nil
}

func (a *APICall) executeServiceCall(ctx context.Context, apiCall *kyvernov1.APICall) ([]byte, error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.entry.Name)
	}

	client, err := a.buildHTTPClient(apiCall.Service)
	if err != nil {
		return nil, err
	}

	req, err := a.buildHTTPRequest(ctx, apiCall)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for APICall %s: %w", a.entry.Name, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request for APICall %s: %w", a.entry.Name, err)
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
			return nil, fmt.Errorf("response length must be less than max allowed response length of %d", a.config.maxAPICallResponseLength)
		} else {
			return nil, fmt.Errorf("failed to read data from APICall %s: %w", a.entry.Name, err)
		}
	}

	a.logger.Info("executed service APICall", "name", a.entry.Name, "len", len(body))
	return body, nil
}

func (a *APICall) buildHTTPRequest(ctx context.Context, apiCall *kyvernov1.APICall) (req *http.Request, err error) {
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

	return nil, fmt.Errorf("invalid request type %s for APICall %s", apiCall.Method, a.entry.Name)
}

func (a *APICall) getToken() string {
	fileName := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	b, err := os.ReadFile(fileName)
	if err != nil {
		a.logger.Info("failed to read service account token", "path", fileName)
		return ""
	}

	return string(b)
}

func (a *APICall) buildHTTPClient(service *kyvernov1.ServiceCall) (*http.Client, error) {
	if service == nil || service.CABundle == "" {
		return http.DefaultClient, nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(service.CABundle)); !ok {
		return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall %s", a.entry.Name)
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

func (a *APICall) buildRequestData(data []kyvernov1.RequestData) (io.Reader, error) {
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

func (a *APICall) transformAndStore(jsonData []byte) ([]byte, error) {
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

func (a *APICall) applyJMESPathJSON(jmesPath string, jsonData []byte) (interface{}, error) {
	var data interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(jsonData), err)
	}
	return a.jp.Search(jmesPath, data)
}
