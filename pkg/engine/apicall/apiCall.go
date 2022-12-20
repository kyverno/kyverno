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
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/pkg/errors"
)

type apiCall struct {
	log     logr.Logger
	entry   kyvernov1.ContextEntry
	ctx     goctx.Context
	jsonCtx context.EvalInterface
	client  dclient.Interface
}

func New(ctx goctx.Context, entry kyvernov1.ContextEntry, jsonCtx context.EvalInterface, client dclient.Interface, log logr.Logger) (*apiCall, error) {
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

	if call.URLPath != "" {
		return a.executeK8sAPICall(call.JMESPath)
	}

	return a.executeServiceCall(call.Service)
}

func (a *apiCall) executeK8sAPICall(path string) ([]byte, error) {
	jsonData, err := a.client.RawAbsPath(a.ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource with raw url\n: %s: %v", path, err)
	}

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
		return nil, errors.Wrapf(err, "failed to build HTTP request for APICall %s", a.entry.Name)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute HTTP request for APICall %s", a.entry.Name)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read data from APICall %s", a.entry.Name)
	}

	return body, nil
}

func (a *apiCall) buildHTTPRequest(service *kyvernov1.ServiceCall) (req *http.Request, err error) {
	token := a.getToken()
	defer func() {
		if token != "" && req != nil {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}()

	if service.RequestType == "GET" {
		req, err = http.NewRequest("GET", service.URL, nil)
		return
	}

	if service.RequestType == "POST" {
		data, dataErr := a.buildPostData(service.Data)
		if dataErr != nil {
			return nil, dataErr
		}

		req, err = http.NewRequest("POST", service.URL, data)
		return
	}

	return nil, fmt.Errorf("invalid request type %s for APICall %s", service.RequestType, a.entry.Name)
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
		return nil, errors.Wrapf(err, "failed to encode HTTP POST data %v for APICall %s", dataMap, a.entry.Name)
	}

	return buffer, nil
}
