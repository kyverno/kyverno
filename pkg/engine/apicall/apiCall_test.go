package apicall

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"gotest.tools/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

var (
	jp        = jmespath.New(config.NewDefaultConfiguration(false))
	apiConfig = APICallConfiguration{
		maxAPICallResponseLength: 1 * 1000 * 1000,
	}
	apiConfigMaxSizeExceed = APICallConfiguration{
		maxAPICallResponseLength: 10,
	}
	apiConfigWithoutSecurityCheck = APICallConfiguration{
		maxAPICallResponseLength: 0,
	}
)

func buildTestServer(responseData []byte, useChunked bool) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer 1234567890" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				http.Error(w, "StatusUnsupportedMediaType", http.StatusUnsupportedMediaType)
				return
			}

			if useChunked {
				flusher, ok := w.(http.Flusher)
				if !ok {
					http.Error(w, "expected http.ResponseWriter to be an http.Flusher", http.StatusInternalServerError)
					return
				}
				chunkSize := len(responseData) / 10
				for i := 0; i < 10; i++ {
					data := responseData[i*chunkSize : (i+1)*chunkSize]
					w.Write(data)
					flusher.Flush()
				}
				w.Write(responseData[10*chunkSize:])
				flusher.Flush()
			} else {
				w.Write(responseData)
			}

			return
		}

		if r.Method == "POST" {
			defer r.Body.Close()
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		}
	})

	return httptest.NewServer(mux)
}

func Test_serviceGetRequest(t *testing.T) {
	testfn := func(t *testing.T, useChunked bool) {
		serverResponse := []byte(`{ "day": "Sunday" }`)
		s := buildTestServer(serverResponse, useChunked)
		defer s.Close()

		entry := kyvernov1.ContextEntry{}
		ctx := enginecontext.NewContext(jp)

		_, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
		assert.ErrorContains(t, err, "missing APICall")

		entry.Name = "test"
		entry.APICall = &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Service: &kyvernov1.ServiceCall{
					URL: s.URL,
					Headers: []kyvernov1.HTTPHeader{
						{Key: "Authorization", Value: "Bearer 1234567890"},
						{Key: "Content-Type", Value: "application/json"},
					},
				},
			},
		}

		call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "invalid request type")

		entry.APICall.Method = "GET"
		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "HTTP 404")

		entry.APICall.Service.URL = s.URL + "/resource"
		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
		assert.NilError(t, err)

		data, err := call.FetchAndLoad(context.TODO())
		assert.NilError(t, err)
		assert.Assert(t, data != nil, "nil data")
		assert.Equal(t, string(serverResponse), string(data))

		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfigMaxSizeExceed, "")
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "response length must be less than max allowed response length of 10")

		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfigWithoutSecurityCheck, "")
		assert.NilError(t, err)
		data, err = call.FetchAndLoad(context.TODO())
		assert.NilError(t, err)
		assert.Assert(t, data != nil, "nil data")
		assert.Equal(t, string(serverResponse), string(data))
	}

	t.Run("Standard", func(t *testing.T) { testfn(t, false) })
	t.Run("Chunked", func(t *testing.T) { testfn(t, true) })
}

func Test_servicePostRequest(t *testing.T) {
	serverResponse := []byte(`{ "day": "Monday" }`)
	s := buildTestServer(serverResponse, false)
	defer s.Close()

	entry := kyvernov1.ContextEntry{
		Name: "test",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Method: "POST",
				Service: &kyvernov1.ServiceCall{
					URL: s.URL + "/resource",
				},
			},
		},
	}

	ctx := enginecontext.NewContext(jp)
	call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
	assert.NilError(t, err)
	data, err := call.FetchAndLoad(context.TODO())
	assert.NilError(t, err)
	assert.Equal(t, "{}\n", string(data))

	imageData := `{
		"containers": {
		  "tomcat": {
			"reference": "https://ghcr.io/tomcat/tomcat:9",
			"registry": "https://ghcr.io",
			"path": "tomcat",
			"name": "tomcat",
			"tag": "9"
		  }
		},
		"initContainers": {
		  "vault": {
			"reference": "https://ghcr.io/vault/vault:v3",
			"registry": "https://ghcr.io",
			"path": "vault",
			"name": "vault",
			"tag": "v3"
		  }
		},
		"ephemeralContainers": {
			"vault": {
			  "reference": "https://ghcr.io/busybox/busybox:latest",
			  "registry": "https://ghcr.io",
			  "path": "busybox",
			  "name": "busybox",
			  "tag": "latest"
			}
		  }
	  }`

	err = ctx.AddContextEntry("images", []byte(imageData))
	assert.NilError(t, err)

	entry.APICall.Data = []kyvernov1.RequestData{
		{
			Key: "images",
			Value: &apiextensionsv1.JSON{
				Raw: []byte("\"{{ images.[containers, initContainers, ephemeralContainers][].*.reference[] }}\""),
			},
		},
	}

	call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
	assert.NilError(t, err)
	data, err = call.FetchAndLoad(context.TODO())
	assert.NilError(t, err)

	expectedResults := `{"images":["https://ghcr.io/tomcat/tomcat:9","https://ghcr.io/vault/vault:v3","https://ghcr.io/busybox/busybox:latest"]}`
	assert.Equal(t, string(expectedResults)+"\n", string(data))
}

func Test_fallbackToDefault(t *testing.T) {
	serverResponse := []byte(`Error from server (NotFound): the server could not find the requested resource`)
	defaultResponse := []byte(`{ "day": "Monday" }`)
	s := buildTestServer(serverResponse, false)
	defer s.Close()

	entry := kyvernov1.ContextEntry{}
	ctx := enginecontext.NewContext(jp)

	entry.Name = "test"
	entry.APICall = &kyvernov1.ContextAPICall{
		APICall: kyvernov1.APICall{
			Service: &kyvernov1.ServiceCall{
				URL: s.URL,
				Headers: []kyvernov1.HTTPHeader{
					{Key: "Authorization", Value: "Bearer 1234567890"},
					{Key: "Content-Type", Value: "application/json"},
				},
			},
		},
		Default: &apiextensionsv1.JSON{
			Raw: defaultResponse,
		},
	}

	entry.APICall.Method = "GET"
	call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
	assert.NilError(t, err)

	jsonData, err := call.Fetch(context.TODO())
	assert.NilError(t, err)
	data, err := call.Store(jsonData)

	assert.NilError(t, err) // no error because it should fallback to default value
	assert.Equal(t, string(defaultResponse), string(data))
}

func buildEchoHeaderTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			responseData := make(map[string][]string)
			for k, v := range r.Header {
				responseData[k] = v
			}
			responseBytes, err := json.Marshal(responseData)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			w.Write(responseBytes)
		}
	})
	return httptest.NewServer(mux)
}

func Test_serviceHeaders(t *testing.T) {
	s := buildEchoHeaderTestServer()
	defer s.Close()

	entry := kyvernov1.ContextEntry{}
	ctx := enginecontext.NewContext(jp)

	entry.Name = "test"
	entry.APICall = &kyvernov1.ContextAPICall{
		APICall: kyvernov1.APICall{
			Method: "GET",
			Service: &kyvernov1.ServiceCall{
				URL: s.URL + "/resource",
				Headers: []kyvernov1.HTTPHeader{
					{Key: "Content-Type", Value: "application/json"},
					{Key: "Custom-Key", Value: "CustomVal"},
				},
			},
		},
	}

	entry.APICall.Service.URL = s.URL + "/resource"
	call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig, "")
	assert.NilError(t, err)
	data, err := call.FetchAndLoad(context.TODO())
	assert.NilError(t, err)
	assert.Assert(t, data != nil, "nil data")

	var responseHeaders map[string][]string
	err = json.Unmarshal(data, &responseHeaders)
	assert.NilError(t, err)
	assert.Equal(t, 4, len(responseHeaders))
	assert.Equal(t, "application/json", responseHeaders["Content-Type"][0])
	assert.Equal(t, "CustomVal", responseHeaders["Custom-Key"][0])
}

type mockClient struct{}

func (c *mockClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return []byte("{}"), nil
}

func Test_CrossNamespaceAccess(t *testing.T) {
	entry := kyvernov1.ContextEntry{
		Name: "test",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "/api/v1/namespaces/kube-system/secrets/top-secret",
				Method:  "GET",
			},
		},
	}
	ctx := enginecontext.NewContext(jp)
	client := &mockClient{}

	// Namespaced policy in 'default' trying to access 'kube-system' - should fail
	call, err := New(logr.Discard(), jp, entry, ctx, client, apiConfig, "default")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.ErrorContains(t, err, "refers to namespace kube-system, which is different from the policy namespace default")

	// Namespaced policy in 'kube-system' trying to access 'kube-system' - should pass
	call, err = New(logr.Discard(), jp, entry, ctx, client, apiConfig, "kube-system")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.NilError(t, err)

	// Namespaced policy in 'default' trying to access a cluster-scoped resource - should fail
	clusterScopedEntry := kyvernov1.ContextEntry{
		Name: "test",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "/api/v1/nodes",
				Method:  "GET",
			},
		},
	}
	call, err = New(logr.Discard(), jp, clusterScopedEntry, ctx, client, apiConfig, "default")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.ErrorContains(t, err, "does not contain a namespace segment, which is required for namespaced policies")

	// ClusterPolicy (empty namespace) accessing any namespace - should pass
	call, err = New(logr.Discard(), jp, entry, ctx, client, apiConfig, "")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.NilError(t, err)
}

func Test_CrossNamespaceAccess_WithVariableSubstitution(t *testing.T) {
	// URLPath with a variable that will be substituted
	entry := kyvernov1.ContextEntry{
		Name: "test",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "/api/v1/namespaces/{{ targetNs }}/secrets/mysecret",
				Method:  "GET",
			},
		},
	}
	ctx := enginecontext.NewContext(jp)
	client := &mockClient{}

	// Set up context so variable resolves to 'kube-system'
	err := ctx.AddContextEntry("targetNs", []byte(`"kube-system"`))
	assert.NilError(t, err)

	// Policy in 'default' - variable resolves to 'kube-system' - should fail
	call, err := New(logr.Discard(), jp, entry, ctx, client, apiConfig, "default")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.ErrorContains(t, err, "refers to namespace kube-system, which is different from the policy namespace default")

	// Policy in 'kube-system' - variable resolves to 'kube-system' - should pass
	call, err = New(logr.Discard(), jp, entry, ctx, client, apiConfig, "kube-system")
	assert.NilError(t, err)
	_, err = call.Fetch(context.TODO())
	assert.NilError(t, err)
}

func Test_contextCancellation(t *testing.T) {
	// Server that delays response longer than our context timeout
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer s.Close()

	entry := kyvernov1.ContextEntry{
		Name: "test",
		APICall: &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Method: "GET",
				Service: &kyvernov1.ServiceCall{
					URL: s.URL,
				},
			},
		},
	}

	// Create a context that will timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	engineCtx := enginecontext.NewContext(jp)
	call, err := New(logr.Discard(), jp, entry, engineCtx, nil, apiConfig, "")
	assert.NilError(t, err)

	_, err = call.FetchAndLoad(ctx)
	assert.Assert(t, err != nil, "expected error due to context cancellation")
	assert.Assert(t, context.DeadlineExceeded == ctx.Err(), "context should be cancelled")
}

func Test_APICallConfiguration_GetTimeout(t *testing.T) {
	// Default timeout via constructor
	config := NewAPICallConfiguration(1000, 30*time.Second)
	assert.Equal(t, 30*time.Second, config.GetTimeout())

	// Custom timeout configuration
	customTimeout := 10 * time.Second
	config = NewAPICallConfiguration(1000, customTimeout)
	assert.Equal(t, customTimeout, config.GetTimeout())

	// Zero timeout means no timeout (as documented in flag help text)
	config = APICallConfiguration{maxAPICallResponseLength: 1000, timeout: 0}
	assert.Equal(t, time.Duration(0), config.GetTimeout())
}
