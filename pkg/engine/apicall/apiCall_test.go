package apicall

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

		_, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
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

		call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "invalid request type")

		entry.APICall.Method = "GET"
		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "HTTP 404")

		entry.APICall.Service.URL = s.URL + "/resource"
		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
		assert.NilError(t, err)

		data, err := call.FetchAndLoad(context.TODO())
		assert.NilError(t, err)
		assert.Assert(t, data != nil, "nil data")
		assert.Equal(t, string(serverResponse), string(data))

		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfigMaxSizeExceed)
		assert.NilError(t, err)
		_, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "response length must be less than max allowed response length of 10")

		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfigWithoutSecurityCheck)
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
	call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
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

	call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
	assert.NilError(t, err)
	data, err = call.FetchAndLoad(context.TODO())
	assert.NilError(t, err)

	expectedResults := `{"images":["https://ghcr.io/tomcat/tomcat:9","https://ghcr.io/vault/vault:v3","https://ghcr.io/busybox/busybox:latest"]}`
	assert.Equal(t, string(expectedResults)+"\n", string(data))
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
	call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
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
