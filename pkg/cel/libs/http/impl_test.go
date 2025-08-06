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
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
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
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", ContextType),
		Lib(),
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
		"http": Context{&contextImpl{
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
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", ContextType),
		Lib(),
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
		"http": Context{&contextImpl{
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

func Test_impl_get_request_with_client_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("http", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 3 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("http://localhost:8080"), env.CELTypeAdapter().NativeToValue(make(map[string]string, 0))},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'http.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), env.CELTypeAdapter().NativeToValue(make(map[string]string, 0))},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}, {
		name: "bad arg 3",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("http://localhost:8080"), types.Bool(false)},
		want: types.NewErr("invalid arg 2: type conversion error from bool to 'map[string]string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.get_request_with_client_string(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_impl_post_request(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", ContextType),
		Lib(),
	}
	env, err := base.Extend(options...)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	ast, issues := env.Compile(`http.Post("http://localhost:8080", { "key": dyn("value"), "foo": dyn(2) })`)
	fmt.Println(issues.String())
	assert.Nil(t, issues)
	assert.NotNil(t, ast)
	prog, err := env.Program(ast)
	assert.NoError(t, err)
	assert.NotNil(t, prog)
	out, _, err := prog.Eval(map[string]any{
		"http": Context{&contextImpl{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "POST")

					var data any
					err := json.NewDecoder(req.Body).Decode(&data)
					assert.NoError(t, err)
					assert.Equal(t, data.(map[string]any)["key"], "value")
					assert.Equal(t, data.(map[string]any)["foo"], float64(2))

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
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("http", ContextType),
		Lib(),
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
		"http": Context{&contextImpl{
			client: testClient{
				doFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, req.URL.String(), "http://localhost:8080")
					assert.Equal(t, req.Method, "POST")
					assert.Equal(t, req.Header.Get("Authorization"), "Bearer token")

					var data any
					err := json.NewDecoder(req.Body).Decode(&data)
					assert.NoError(t, err)
					assert.Equal(t, data.(map[string]any)["key"], "value")

					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"body": "ok"}`))}, nil
				},
			},
		}},
	})
	assert.NoError(t, err)
	body := out.Value().(map[string]any)
	assert.Equal(t, body["body"], "ok")
}

func Test_impl_post_request_string_with_client_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("http", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "not enough args",
		args: nil,
		want: types.NewErr("expected 4 arguments, got %d", 0),
	}, {
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("http://localhost:8080"), types.String("payload"), env.CELTypeAdapter().NativeToValue(make(map[string]string, 0))},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'http.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false), types.String("payload"), env.CELTypeAdapter().NativeToValue(make(map[string]string, 0))},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
		// }, {
		// 	name: "bad arg 3",
		// 	args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("http://localhost:8080"), env.CELTypeAdapter().NativeToValue(Context{}), env.CELTypeAdapter().NativeToValue(make(map[string]string, 0))},
		// 	want: types.NewErr("invalid arg 3: type conversion error from bool to 'map[string]string'"),
	}, {
		name: "bad arg 4",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.String("http://localhost:8080"), types.String("payload"), types.Bool(false)},
		want: types.NewErr("invalid arg 3: type conversion error from bool to 'map[string]string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.post_request_string_with_client(tt.args...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_impl_http_client_string(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	options := []cel.EnvOption{
		cel.Variable("pem", types.StringType),
		cel.Variable("http", ContextType),
		Lib(),
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
		"http": Context{&contextImpl{}},
	})
	assert.NoError(t, err)
	reqProvider := out.Value().(*contextImpl)
	assert.NotNil(t, reqProvider)
}

func Test_impl_http_client_string_error(t *testing.T) {
	base, err := compiler.NewBaseEnv()
	assert.NoError(t, err)
	assert.NotNil(t, base)
	env, err := base.Extend(
		cel.Variable("http", ContextType),
		Lib(),
	)
	assert.NoError(t, err)
	assert.NotNil(t, env)
	tests := []struct {
		name string
		args []ref.Val
		want ref.Val
	}{{
		name: "bad arg 1",
		args: []ref.Val{types.String("foo"), types.String("http://localhost:8080"), types.String("caBundle")},
		want: types.NewErr("invalid arg 0: unsupported native conversion from string to 'http.Context'"),
	}, {
		name: "bad arg 2",
		args: []ref.Val{env.CELTypeAdapter().NativeToValue(Context{}), types.Bool(false)},
		want: types.NewErr("invalid arg 1: type conversion error from bool to 'string'"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &impl{}
			got := c.http_client_string(tt.args[0], tt.args[1])
			assert.Equal(t, tt.want, got)
		})
	}
}
