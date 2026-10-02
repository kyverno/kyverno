package evaluator

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
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

// tokenRegistry wraps an in-memory OCI registry in the bearer token flow used by
// registries such as GitLab, GHCR and Docker Hub, counting how often a client is
// challenged and exchanges the challenge for a token.
type tokenRegistry struct {
	inner       http.Handler
	url         func() string
	pings       atomic.Int64
	tokens      atomic.Int64
	rejectToken atomic.Bool
}

func (t *tokenRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		t.tokens.Add(1)
		if t.rejectToken.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"token":"test-token"}`)
		return
	}
	if r.URL.Path == "/v2/" {
		t.pings.Add(1)
	}
	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm=%q,service="registry"`, t.url()+"/token"))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	t.inner.ServeHTTP(w, r)
}

// TestEvaluationReusesRegistryAuth asserts that evaluating a policy against one
// request performs a single registry auth handshake per repository, covering the
// image prefetch and the CEL image functions together.
//
// Regression test for #17832.
func TestEvaluationReusesRegistryAuth(t *testing.T) {
	handler := &tokenRegistry{inner: registry.New(registry.Logger(log.New(io.Discard, "", 0)))}
	server := httptest.NewServer(handler)
	defer server.Close()
	handler.url = func() string { return server.URL }
	host := strings.TrimPrefix(server.URL, "http://")

	// three tags in one repository: the puller caches per repository, so the whole
	// evaluation must still cost a single handshake
	images := make([]any, 0, 3)
	for _, tag := range []string{"one", "two", "three"} {
		image := fmt.Sprintf("%s/test/image:%s", host, tag)
		ref, err := name.ParseReference(image, name.Insecure)
		require.NoError(t, err)
		pushed, err := random.Image(256, 2)
		require.NoError(t, err)
		require.NoError(t, remote.Write(ref, pushed, remote.WithAuth(authn.Anonymous)))
		images = append(images, image)
	}

	policy := &policiesv1beta1.ImageValidatingPolicy{
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			EvaluationConfiguration:  &policiesv1beta1.EvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false)},
			Credentials:              &policiesv1beta1.Credentials{AllowInsecureRegistry: true},
			ImageExtractors:          []policiesv1beta1.ImageExtractor{{Name: "all", Expression: "object.images"}},
			Validations: []admissionregistrationv1.Validation{
				{Expression: fmt.Sprintf("getImageData(%q).config != null", images[0])},
			},
		},
	}
	compiled, errs := NewCompiler(nil).Compile(policy, nil)
	require.Empty(t, errs)

	tests := []struct {
		name        string
		rejectToken bool
		wantErr     bool
		wantPings   int64
	}{{
		name:      "one handshake serves the whole evaluation",
		wantPings: 1,
	}, {
		// reusing a fetcher must not mask a rejection: the error still surfaces
		name:        "a rejected token exchange fails the evaluation",
		rejectToken: true,
		wantErr:     true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler.rejectToken.Store(tt.rejectToken)
			// a fresh context per case: its cache is request scoped
			ictx, err := imagedataloader.NewImageContext(nil, nil, nil)
			require.NoError(t, err)
			rt := &imageverify.Runtime{ImageContext: ictx, Results: imageverify.NewImageVerificationResults()}

			handler.pings.Store(0)
			handler.tokens.Store(0)
			result, err := compiled.Evaluate(context.Background(), rt, nil, map[string]any{"images": images}, nil, false, nil, nil)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Result, result.Message)
			require.Equal(t, tt.wantPings, handler.pings.Load(), "expected a single /v2/ ping for the whole evaluation")
			require.Equal(t, tt.wantPings, handler.tokens.Load(), "expected a single token exchange for the whole evaluation")
		})
	}
}
