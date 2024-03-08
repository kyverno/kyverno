package apicall

import (
	"context"
	"fmt"
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

func buildTestServer(t *testing.T, responseData []byte, useChunked bool, verifyRequest func(t *testing.T, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		if verifyRequest != nil {
			verifyRequest(t, r)
		}

		if r.Method == "GET" {
			w.Write(responseData)

			if useChunked {
				flusher, ok := w.(http.Flusher)
				if !ok {
					panic("expected http.ResponseWriter to be an http.Flusher")
				}
				for i := 1; i <= 10; i++ {
					fmt.Fprintf(w, "Chunk #%d\n", i)
					flusher.Flush()
				}
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
		s := buildTestServer(t, serverResponse, false, nil)
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
		data, err = call.FetchAndLoad(context.TODO())
		assert.ErrorContains(t, err, "response length must be less than max allowed response length of 10.")

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
	s := buildTestServer(t, serverResponse, false, nil)
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

func test_serviceRequestHeaders(t *testing.T, headers []kyvernov1.HeaderData, verifyRequest func(t *testing.T, r *http.Request)) func(t *testing.T, useChunked bool) {
	return func(t *testing.T, useChunked bool) {
		serverResponse := []byte(`{ "day": "Sunday" }`)
		s := buildTestServer(t, serverResponse, false, verifyRequest)
		defer s.Close()

		entry := kyvernov1.ContextEntry{}
		ctx := enginecontext.NewContext(jp)

		_, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
		assert.ErrorContains(t, err, "missing APICall")

		entry.Name = "test"
		entry.APICall = &kyvernov1.ContextAPICall{
			APICall: kyvernov1.APICall{
				Headers: headers,
				Service: &kyvernov1.ServiceCall{
					URL: s.URL,
				},
			},
		}

		call, err := New(logr.Discard(), jp, entry, ctx, nil, apiConfig)

		entry.APICall.Method = "GET"
		entry.APICall.Service.URL = s.URL + "/resource"
		call, err = New(logr.Discard(), jp, entry, ctx, nil, apiConfig)
		assert.NilError(t, err)

		data, err := call.FetchAndLoad(context.TODO())
		assert.NilError(t, err)
		assert.Assert(t, data != nil, "nil data")
		assert.Equal(t, string(serverResponse), string(data))
	}
}

func Test_serviceRequestCustomHeaders(t *testing.T) {
	var singleHeader = []kyvernov1.HeaderData{
		{
			Key:   "foo",
			Value: "bar",
		},
	}

	testFunc := test_serviceRequestHeaders(t, singleHeader, func(t *testing.T, r *http.Request) {
		assert.Equal(t, "bar", r.Header.Get("foo"))
	})
	t.Run("Single Header", func(t *testing.T) { testFunc(t, false) })

	var multipleHeader = []kyvernov1.HeaderData{
		{
			Key:   "foo",
			Value: "bar",
		},
		{
			Key:   "User-Agent",
			Value: "Kyverno Test",
		},
	}

	testFunc = test_serviceRequestHeaders(t, multipleHeader, func(t *testing.T, r *http.Request) {
		assert.Equal(t, "bar", r.Header.Get("foo"))
		assert.Equal(t, "Kyverno Test", r.Header.Get("User-Agent"))
	})
	t.Run("Multiple Header", func(t *testing.T) { testFunc(t, false) })

	var duplicateHeader = []kyvernov1.HeaderData{
		{
			Key:   "foo",
			Value: "bar",
		},
		{
			Key:   "foo",
			Value: "baz",
		},
	}

	testFunc = test_serviceRequestHeaders(t, duplicateHeader, func(t *testing.T, r *http.Request) {
		headerValues := r.Header.Values("foo")
		assert.Equal(t, 2, len(headerValues))
		assert.Equal(t, "bar", headerValues[0])
		assert.Equal(t, "baz", headerValues[1])
	})
	t.Run("Duplicate Header", func(t *testing.T) { testFunc(t, false) })

	var overrideAuthorizationHeader = []kyvernov1.HeaderData{
		{
			Key:   "Authorization",
			Value: "abc12345",
		},
	}

	testFunc = test_serviceRequestHeaders(t, overrideAuthorizationHeader, func(t *testing.T, r *http.Request) {
		assert.Equal(t, "abc12345", r.Header.Get("Authorization"))
	})
	t.Run("Authorization Override", func(t *testing.T) { testFunc(t, false) })
}
