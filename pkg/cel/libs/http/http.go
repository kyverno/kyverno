package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type clientInterface interface {
	Do(*http.Request) (*http.Response, error)
}

type httpProvider struct {
	client clientInterface
}

func (r *httpProvider) Get(url string, headers map[string]string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(context.TODO(), "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	for h, v := range headers {
		req.Header.Add(h, v)
	}
	return r.executeRequest(r.client, req)
}

func (r *httpProvider) Post(url string, data map[string]any, headers map[string]string) (map[string]any, error) {
	body, err := buildRequestData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request data: %v", err)
	}
	req, err := http.NewRequestWithContext(context.TODO(), "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	for h, v := range headers {
		req.Header.Add(h, v)
	}
	return r.executeRequest(r.client, req)
}

func (r *httpProvider) executeRequest(client clientInterface, req *http.Request) (map[string]any, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	body := make(map[string]any)
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("Unable to decode JSON body %v", err)
	}
	return body, nil
}

func (r *httpProvider) Client(caBundle string) (HttpInterface, error) {
	if caBundle == "" {
		return r, nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(caBundle)); !ok {
		return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall")
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		},
	}
	return &httpProvider{
		client: &http.Client{
			Transport: tracing.Transport(transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan)),
		},
	}, nil
}

func buildRequestData(data map[string]any) (io.Reader, error) {
	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(data); err != nil {
		return nil, fmt.Errorf("failed to encode HTTP POST data %v: %w", data, err)
	}
	return buffer, nil
}
