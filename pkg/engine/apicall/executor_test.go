package apicall

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockClientWithError struct {
	err error
}

func (c *mockClientWithError) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, c.err
}

func (c *mockClientWithError) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("not implemented")
}

func Test_ExecuteK8sAPICall_ForbiddenError(t *testing.T) {
	// Create a Forbidden error similar to what K8s API returns
	forbiddenErr := apierrors.NewForbidden(
		schema.GroupResource{Group: "storage.k8s.io", Resource: "volumeattachments"},
		"",
		errors.New("User \"system:serviceaccount:kyverno:kyverno-admission-controller\" cannot list resource \"volumeattachments\" in API group \"storage.k8s.io\" at the cluster scope"),
	)

	client := &mockClientWithError{err: forbiddenErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig, "")

	call := &kyvernov1.APICall{
		URLPath: "/apis/storage.k8s.io/v1/volumeattachments",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "permission denied")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	assert.ErrorContains(t, err, "/apis/storage.k8s.io/v1/volumeattachments")
}

func Test_ExecuteK8sAPICall_UnauthorizedError(t *testing.T) {
	// Create an Unauthorized error similar to what K8s API returns
	unauthorizedErr := apierrors.NewUnauthorized("access denied")

	client := &mockClientWithError{err: unauthorizedErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig, "")

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "permission denied")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	assert.ErrorContains(t, err, "/api/v1/namespaces")
}

func Test_ExecuteK8sAPICall_OtherError(t *testing.T) {
	// Create a NotFound error (non-permission error)
	notFoundErr := apierrors.NewNotFound(
		schema.GroupResource{Group: "", Resource: "configmaps"},
		"test-config",
	)

	client := &mockClientWithError{err: notFoundErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig, "")

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces/default/configmaps/test-config",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	// Should NOT contain "permission denied" prefix for non-permission errors
	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	// Verify it doesn't have permission denied prefix
	errMsg := err.Error()
	assert.Check(t, !contains(errMsg, "permission denied"))
}

func Test_ExecuteK8sAPICall_GenericError(t *testing.T) {
	// Generic error that's not a K8s API error
	genericErr := errors.New("connection timeout")

	client := &mockClientWithError{err: genericErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig, "")

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/pods",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "connection timeout")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	// Verify it doesn't have permission denied prefix
	errMsg := err.Error()
	assert.Check(t, !contains(errMsg, "permission denied"))
}

func Test_ExecuteK8sAPICall_Success(t *testing.T) {
	client := &mockClient{}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig, "")

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces",
		Method:  "GET",
	}

	data, err := executor.Execute(context.TODO(), call)
	assert.NilError(t, err)
	assert.Equal(t, string(data), "{}")
}

func Test_ExecuteServiceCall_AllowsMissingScopedTokenWhenAuthorizationMissing(t *testing.T) {
	missingTokenPath := scopedTokenPath + ".missing"
	oldPath := scopedTokenPath
	scopedTokenPath = missingTokenPath
	t.Cleanup(func() {
		scopedTokenPath = oldPath
	})

	var gotAuth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	executor := NewExecutor(logr.Discard(), "test-call", &mockClient{}, apiConfig, "")
	call := &kyvernov1.APICall{
		Method: "GET",
		Service: &kyvernov1.ServiceCall{
			URL: s.URL,
		},
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.NilError(t, err)
	assert.Equal(t, gotAuth, "")
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockClientWithResources struct {
	mockClientWithError
	secrets    map[string]*unstructured.Unstructured
	configmaps map[string]*unstructured.Unstructured
}

func (c *mockClientWithResources) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return []byte("{}"), nil
}

func (c *mockClientWithResources) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	key := namespace + "/" + name
	switch kind {
	case "Secret":
		if obj, ok := c.secrets[key]; ok {
			return obj, nil
		}
	case "ConfigMap":
		if obj, ok := c.configmaps[key]; ok {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("%s %q not found", kind, key)
}

func newSecret(namespace, name string, data map[string]string) *unstructured.Unstructured {
	obj := map[string]interface{}{"data": map[string]interface{}{}}
	for k, v := range data {
		obj["data"].(map[string]interface{})[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func newConfigMap(namespace, name string, data map[string]string) *unstructured.Unstructured {
	obj := map[string]interface{}{"data": map[string]interface{}{}}
	for k, v := range data {
		obj["data"].(map[string]interface{})[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func Test_resolveValueFrom_SecretHeader(t *testing.T) {
	// base64("user:pass") = "dXNlcjpwYXNz"
	client := &mockClientWithResources{
		secrets: map[string]*unstructured.Unstructured{
			"kyverno/api-creds": newSecret("kyverno", "api-creds", map[string]string{
				"basic-auth": base64.StdEncoding.EncodeToString([]byte("user:pass")),
			}),
		},
	}
	exec := NewExecutor(logr.Discard(), "test", client, apiConfig, "")

	val, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		SecretKeyRef: &kyvernov1.SecretKeySelector{
			Name:      "api-creds",
			Namespace: "kyverno",
			Key:       "basic-auth",
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, "user:pass", val)
}

func Test_resolveValueFrom_ConfigMapHeader(t *testing.T) {
	client := &mockClientWithResources{
		configmaps: map[string]*unstructured.Unstructured{
			"kyverno/api-config": newConfigMap("kyverno", "api-config", map[string]string{
				"api-key": "my-plain-api-key",
			}),
		},
	}
	exec := NewExecutor(logr.Discard(), "test", client, apiConfig, "")

	val, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		ConfigMapKeyRef: &kyvernov1.ConfigMapKeySelector{
			Name:      "api-config",
			Namespace: "kyverno",
			Key:       "api-key",
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, "my-plain-api-key", val)
}

func Test_resolveValueFrom_BothRefsError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	_, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		SecretKeyRef:    &kyvernov1.SecretKeySelector{Name: "s", Namespace: "ns", Key: "k"},
		ConfigMapKeyRef: &kyvernov1.ConfigMapKeySelector{Name: "c", Namespace: "ns", Key: "k"},
	})
	assert.ErrorContains(t, err, "only one of secretKeyRef or configMapKeyRef")
}

func Test_resolveValueFrom_NoRefError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	_, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{})
	assert.ErrorContains(t, err, "one of secretKeyRef or configMapKeyRef must be specified")
}

func Test_resolveValueFrom_CrossNamespaceError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "my-namespace")

	_, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		SecretKeyRef: &kyvernov1.SecretKeySelector{
			Name:      "creds",
			Namespace: "other-namespace",
			Key:       "token",
		},
	})
	assert.ErrorContains(t, err, "cross-namespace reference not allowed")
}

func Test_resolveValueFrom_NamespaceDefaultsToPolicy(t *testing.T) {
	client := &mockClientWithResources{
		secrets: map[string]*unstructured.Unstructured{
			"my-namespace/creds": newSecret("my-namespace", "creds", map[string]string{
				"token": base64.StdEncoding.EncodeToString([]byte("secret-value")),
			}),
		},
	}
	exec := NewExecutor(logr.Discard(), "test", client, apiConfig, "my-namespace")

	val, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		SecretKeyRef: &kyvernov1.SecretKeySelector{
			Name: "creds",
			// Namespace omitted — should default to "my-namespace"
			Key: "token",
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, "secret-value", val)
}

func Test_resolveValueFrom_ClusterScopedRequiresNamespace(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	_, err := exec.resolveValueFrom(context.TODO(), &kyvernov1.SecretOrConfigMapSource{
		SecretKeyRef: &kyvernov1.SecretKeySelector{
			Name: "creds",
			// Namespace omitted in cluster-scoped context — must error
			Key: "token",
		},
	})
	assert.ErrorContains(t, err, "namespace is required")
}

func Test_buildHTTPClient_CABundleFromConfigMap(t *testing.T) {
	client := &mockClientWithResources{
		configmaps: map[string]*unstructured.Unstructured{
			"kyverno/ca-bundle": newConfigMap("kyverno", "ca-bundle", map[string]string{
				"ca.crt": "",
			}),
		},
	}
	exec := NewExecutor(logr.Discard(), "test", client, apiConfig, "")

	httpClient, err := exec.buildHTTPClient(context.TODO(), &kyvernov1.ServiceCall{
		URL: "https://example.com",
		CABundleFrom: &kyvernov1.SecretOrConfigMapSource{
			ConfigMapKeyRef: &kyvernov1.ConfigMapKeySelector{
				Name:      "ca-bundle",
				Namespace: "kyverno",
				Key:       "ca.crt",
			},
		},
	})
	// Empty CA bundle → returns plain http.Client (no TLS config needed)
	assert.NilError(t, err)
	assert.Assert(t, httpClient != nil)
}

func Test_buildHTTPClient_BothCABundleError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	_, err := exec.buildHTTPClient(context.TODO(), &kyvernov1.ServiceCall{
		URL:      "https://example.com",
		CABundle: "some-pem-content",
		CABundleFrom: &kyvernov1.SecretOrConfigMapSource{
			ConfigMapKeyRef: &kyvernov1.ConfigMapKeySelector{
				Name:      "ca",
				Namespace: "kyverno",
				Key:       "bundle",
			},
		},
	})
	assert.ErrorContains(t, err, "at most one of caBundle or caBundleFrom")
}

func Test_addHTTPHeaders_NeitherValueNorValueFromError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := exec.addHTTPHeaders(context.TODO(), req, []kyvernov1.HTTPHeader{
		{Key: "X-Empty"}, // no Value, no ValueFrom
	})
	assert.ErrorContains(t, err, "exactly one of value or valueFrom")
}

func Test_addHTTPHeaders_BothValueAndValueFromError(t *testing.T) {
	exec := NewExecutor(logr.Discard(), "test", &mockClientWithResources{}, apiConfig, "")

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := exec.addHTTPHeaders(context.TODO(), req, []kyvernov1.HTTPHeader{
		{
			Key:   "Authorization",
			Value: "Bearer hardcoded",
			ValueFrom: &kyvernov1.SecretOrConfigMapSource{
				SecretKeyRef: &kyvernov1.SecretKeySelector{Name: "s", Namespace: "ns", Key: "k"},
			},
		},
	})
	assert.ErrorContains(t, err, "exactly one of value or valueFrom")
}
