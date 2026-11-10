package serve

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/evaluator"
	celhttp "github.com/kyverno/sdk/extensions/cel/libs/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func send(h http.Handler, method, path, host, contentType string, body []byte) (int, Status) {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if host != "" {
		req.Host = host
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var st Status
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	return rec.Code, st
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestWrongMethodIsStatus(t *testing.T) {
	h := newTestServer(t, []evaluator.Mode{evaluator.ModeKubernetes}, fixture("policies", "kubernetes")).Handler()
	tests := []struct{ method, path, allow string }{
		{http.MethodGet, "/v1alpha1/evaluate/Kubernetes", http.MethodPost},
		{http.MethodGet, "/v1alpha1/evaluate", http.MethodPost},
		{http.MethodPost, "/v1alpha1/policies", http.MethodGet},
		{http.MethodDelete, "/healthz", http.MethodGet},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Equal(t, tt.allow, rec.Header().Get("Allow"))
			var st Status
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st), rec.Body.String())
			assert.Equal(t, "Failure", st.Status)
			assert.Equal(t, http.StatusMethodNotAllowed, st.Code)
		})
	}
	code, _ := send(h, http.MethodHead, "/healthz", "", "", nil)
	assert.Equal(t, http.StatusOK, code, "HEAD is accepted where GET is")
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestContentTypeRequired(t *testing.T) {
	h := newTestServer(t, []evaluator.Mode{evaluator.ModeKubernetes}, fixture("policies", "kubernetes")).Handler()
	pod := []byte(read(t, fixture("payloads", "kubernetes", "pod.yaml")))
	request := []byte(mustJSON(t, EvaluationRequest{Mode: "Kubernetes", Payload: string(pod)}))
	tests := []struct {
		name, path, contentType string
		body                    []byte
		code                    int
	}{
		// the CORS "simple" types a web page can send without a preflight
		{"mode route without content type", "/v1alpha1/evaluate/Kubernetes", "", pod, http.StatusUnsupportedMediaType},
		{"mode route text/plain", "/v1alpha1/evaluate/Kubernetes", "text/plain", pod, http.StatusUnsupportedMediaType},
		{"mode route form", "/v1alpha1/evaluate/Kubernetes", "application/x-www-form-urlencoded", pod, http.StatusUnsupportedMediaType},
		{"mode route multipart", "/v1alpha1/evaluate/Kubernetes", "multipart/form-data; boundary=x", pod, http.StatusUnsupportedMediaType},
		{"request route text/plain", "/v1alpha1/evaluate", "text/plain;charset=UTF-8", request, http.StatusUnsupportedMediaType},
		{"request route yaml", "/v1alpha1/evaluate", "application/yaml", request, http.StatusUnsupportedMediaType},
		// raw YAML stays supported on the mode route
		{"mode route yaml", "/v1alpha1/evaluate/Kubernetes", "application/yaml", pod, http.StatusOK},
		{"mode route text/yaml", "/v1alpha1/evaluate/Kubernetes", "text/yaml", pod, http.StatusOK},
		{"mode route json with charset", "/v1alpha1/evaluate/Kubernetes", "application/json; charset=utf-8", pod, http.StatusOK},
		{"request route json", "/v1alpha1/evaluate", "application/json", request, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, st := send(h, http.MethodPost, tt.path, "", tt.contentType, tt.body)
			assert.Equal(t, tt.code, code, st.Message)
			if tt.code == http.StatusUnsupportedMediaType {
				assert.Contains(t, st.Message, "Content-Type must be one of")
			}
		})
	}
}

//nolint:paralleltest // newServerFromPaths sets the process-global CRD processor
func TestLoopbackHost(t *testing.T) {
	s := newTestServer(t, []evaluator.Mode{evaluator.ModeKubernetes}, fixture("policies", "kubernetes"))
	code, _ := send(s.Handler(), http.MethodGet, "/healthz", "attacker.example.com", "", nil)
	assert.Equal(t, http.StatusOK, code, "the Host is not checked unless the server listens on loopback")

	s.loopbackOnly = true
	h := s.Handler()
	for host, want := range map[string]int{
		"127.0.0.1:8080":          http.StatusOK,
		"localhost:8080":          http.StatusOK,
		"LOCALHOST":               http.StatusOK,
		"[::1]:8080":              http.StatusOK,
		"attacker.example.com":    http.StatusForbidden,
		"rebind.example.com:8080": http.StatusForbidden,
		"10.0.0.1:8080":           http.StatusForbidden,
	} {
		code, st := send(h, http.MethodGet, "/healthz", host, "", nil)
		assert.Equal(t, want, code, host)
		if want == http.StatusForbidden {
			assert.Contains(t, st.Message, "is not allowed", host)
		}
	}
}

//nolint:paralleltest // mutates http.DefaultClient and http.DefaultTransport
func TestBoundPolicyHTTP(t *testing.T) {
	transport := http.DefaultTransport.(*http.Transport)
	clientTimeout, headerTimeout := http.DefaultClient.Timeout, transport.ResponseHeaderTimeout
	t.Cleanup(func() {
		http.DefaultClient.Timeout = clientTimeout
		transport.ResponseHeaderTimeout = headerTimeout
	})
	// accepts the connection and never answers until the test ends
	done := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-done }))
	t.Cleanup(hang.Close)
	t.Cleanup(func() { close(done) })

	boundPolicyHTTP(200 * time.Millisecond)
	unguarded := celhttp.NewHTTP()
	// a CIDR blocklist that does not cover the test server makes the library build a guarded
	// client on a clone of http.DefaultTransport, as kyverno does by default
	guarded, err := celhttp.NewHTTPWithBlocklist([]string{"192.0.2.0/24"}, nil)
	require.NoError(t, err)
	for name, ctx := range map[string]celhttp.ContextInterface{"default client": unguarded, "guarded client": guarded} {
		start := time.Now()
		_, err := ctx.Get(hang.URL, nil)
		require.Error(t, err, name)
		assert.Less(t, time.Since(start), 5*time.Second, name)
	}
}
