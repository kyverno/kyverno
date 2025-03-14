package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/assert"
)

var pemExample = `-----BEGIN CERTIFICATE-----
MIICMzCCAZygAwIBAgIJALiPnVsvq8dsMA0GCSqGSIb3DQEBBQUAMFMxCzAJBgNV
BAYTAlVTMQwwCgYDVQQIEwNmb28xDDAKBgNVBAcTA2ZvbzEMMAoGA1UEChMDZm9v
MQwwCgYDVQQLEwNmb28xDDAKBgNVBAMTA2ZvbzAeFw0xMzAzMTkxNTQwMTlaFw0x
ODAzMTgxNTQwMTlaMFMxCzAJBgNVBAYTAlVTMQwwCgYDVQQIEwNmb28xDDAKBgNV
BAcTA2ZvbzEMMAoGA1UEChMDZm9vMQwwCgYDVQQLEwNmb28xDDAKBgNVBAMTA2Zv
bzCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAzdGfxi9CNbMf1UUcvDQh7MYB
OveIHyc0E0KIbhjK5FkCBU4CiZrbfHagaW7ZEcN0tt3EvpbOMxxc/ZQU2WN/s/wP
xph0pSfsfFsTKM4RhTWD2v4fgk+xZiKd1p0+L4hTtpwnEw0uXRVd0ki6muwV5y/P
+5FHUeldq+pgTcgzuK8CAwEAAaMPMA0wCwYDVR0PBAQDAgLkMA0GCSqGSIb3DQEB
BQUAA4GBAJiDAAtY0mQQeuxWdzLRzXmjvdSuL9GoyT3BF/jSnpxz5/58dba8pWen
v3pj4P3w5DoOso0rzkZy2jEsEitlVM2mLSbQpMM+MUVQCQoiG6W9xuCFuxSrwPIS
pAqEAuV4DNoxQKKWmhVv+J0ptMWD25Pnpxeq5sXzghfJnslJlQND
-----END CERTIFICATE-----`

type testClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (t testClient) Do(req *http.Request) (*http.Response, error) {
	return t.doFunc(req)
}

func Test_impl_get_request(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", HTTPType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Get("http://localhost:8080")`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"http": HTTP{&HttpProvider{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "GET")

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"body": "ok"}`))}, nil
				},
			},
		}},
	})
	assert.NoError(t, err)
	body := out.Value().(map[string]any)
	assert.Equal(t, body["body"], "ok")
}

func Test_impl_get_request_with_headers(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", HTTPType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Get("http://localhost:8080", {"Authorization": "Bearer token"})`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"http": HTTP{&HttpProvider{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "GET")
					assert.Equal(t, req.Header.Get("Authorization"), "Bearer token")

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"body": "ok"}`))}, nil
				},
			},
		}},
	})
	assert.NoError(t, err)
	body := out.Value().(map[string]any)
	assert.Equal(t, body["body"], "ok")
}

func Test_impl_post_request(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", HTTPType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Post("http://localhost:8080", {"key": "value"})`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"http": HTTP{&HttpProvider{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "POST")

					data := make(map[string]string, 0)
					json.NewDecoder(req.Body).Decode(&data)
					assert.Equal(t, data["key"], "value")

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"body": "ok"}`))}, nil
				},
			},
		}},
	})
	assert.NoError(t, err)
	body := out.Value().(map[string]any)
	assert.Equal(t, body["body"], "ok")
}

func Test_impl_post_request_with_headers(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", HTTPType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Post("http://localhost:8080", {"key": "value"}, {"Authorization": "Bearer token"})`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"http": HTTP{&HttpProvider{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "POST")
					assert.Equal(t, req.Header.Get("Authorization"), "Bearer token")

					data := make(map[string]string, 0)
					json.NewDecoder(req.Body).Decode(&data)
					assert.Equal(t, data["key"], "value")

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"body": "ok"}`))}, nil
				},
			},
		}},
	})
	assert.NoError(t, err)
	body := out.Value().(map[string]any)
	assert.Equal(t, body["body"], "ok")
}

func Test_impl_http_client_string(t *testing.T) {
	opts := Lib()
	base, err := cel.NewEnv(opts)
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("pem", types.StringType),
		cel.Variable("http", HTTPType),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Client(pem)`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"pem":  pemExample,
		"http": HTTP{&HttpProvider{}},
	})
	assert.NoError(t, err)
	reqProvider := out.Value().(*HttpProvider)
	assert.NotNil(t, reqProvider)
}
