package engine

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
)

// TestHandleValidating_ExtractionMode_SharesRegistryAuth asserts that an
// extraction-mode policy authenticates to a repository once per request, not
// once per pod template: evaluateExtractedIv evaluates the policy once per
// template, and each evaluation would otherwise build a puller of its own.
func TestHandleValidating_ExtractionMode_SharesRegistryAuth(t *testing.T) {
	var pings, tokens atomic.Int64
	inner := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokens.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"token":"test-token"}`)
			return
		}
		if r.URL.Path == "/v2/" {
			pings.Add(1)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service="registry"`, server.URL+"/token"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")

	// one template per image, all in one repository: each template fetches an
	// image the others did not, so none is served from the image context's cache
	jobs := make([]any, 0, 3)
	for i, tag := range []string{"one", "two", "three"} {
		image := fmt.Sprintf("%s/test/image:%s", host, tag)
		ref, err := name.ParseReference(image, name.Insecure)
		require.NoError(t, err)
		pushed, err := random.Image(256, 2)
		require.NoError(t, err)
		require.NoError(t, remote.Write(ref, pushed, remote.WithAuth(authn.Anonymous)))
		jobs = append(jobs, map[string]any{
			"name": fmt.Sprintf("workers-%d", i),
			"template": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
				"containers": []any{map[string]any{"name": "worker", "image": image}},
			}}}},
		})
	}
	jobSet := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "multi-image-jobset", "namespace": "default"},
		"spec":       map[string]any{"replicatedJobs": jobs},
	}}
	raw, err := jobSet.MarshalJSON()
	require.NoError(t, err)

	policy := buildExtractionModeRequestPolicy()
	policy.Spec.Credentials = &policiesv1beta1.Credentials{AllowInsecureRegistry: true}
	policy.Spec.Validations = []admissionregistrationv1.Validation{
		{Expression: "images.containers.all(image, getImageData(image).config != null)"},
	}
	provider, err := NewProvider(iveval.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nsResolver, matching.NewMatcher(), nil, nil, config.NewDefaultConfiguration(false))

	pings.Store(0)
	tokens.Store(0)
	resp, err := eng.HandleValidating(context.Background(), engine.EngineRequest{
		Request: v1.AdmissionRequest{
			Operation:       v1.Create,
			Kind:            metav1.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
			Resource:        metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			RequestResource: &metav1.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			Namespace:       "default",
			Name:            "multi-image-jobset",
			Object:          apiruntime.RawExtension{Raw: raw},
		},
	}, nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	rule := resp.Policies[0].Result
	require.Equal(t, engineapi.RuleStatusPass, rule.Status(), rule.Message())
	require.Equal(t, int64(1), pings.Load(), "expected a single /v2/ ping across all three pod templates")
	require.Equal(t, int64(1), tokens.Load(), "expected a single token exchange across all three pod templates")
}
