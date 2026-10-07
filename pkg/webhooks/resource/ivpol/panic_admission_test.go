package ivpol

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// TestPanicInPolicyDeniesAdmissionWithoutCrashing runs the real webhook handler,
// engine and an in-process registry. The registry transport panics while
// serving the image manifest, which the policy fetches in its own evaluation
// goroutine during the image prefetch, outside cel-go. The Pod must be denied
// with the panic in the message, and the process must keep running.
func TestPanicInPolicyDeniesAdmissionWithoutCrashing(t *testing.T) {
	// DefaultTransport is process-global, so this test must not run in parallel.
	oldTransport := regcreds.DefaultTransport
	transport := &http.Transport{TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}}
	regcreds.DefaultTransport = transport
	t.Cleanup(func() { regcreds.DefaultTransport = oldTransport; transport.CloseIdleConnections() })
	registryHandler := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	panicOnManifest := false
	transport.RegisterProtocol("https", diagnosticTransport(func(r *http.Request) (*http.Response, error) {
		// Only the manifest request panics: go-containerregistry may ping the
		// registry from goroutines of its own, which no recover in kyverno covers.
		if panicOnManifest && strings.Contains(r.URL.Path, "/manifests/") {
			panic("injected panic while serving " + r.URL.Path)
		}
		req := r.Clone(r.Context())
		req.RequestURI = r.URL.RequestURI()
		if req.Body == nil {
			req.Body = http.NoBody
		}
		recorder := httptest.NewRecorder()
		registryHandler.ServeHTTP(recorder, req)
		response := recorder.Result()
		response.Request = r
		return response, nil
	}))
	const image = "registry.example/app:v1"
	ref, err := name.ParseReference(image)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, empty.Image, remote.WithTransport(transport)))
	panicOnManifest = true

	policy := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-signed-images"},
		Spec: policiesv1beta1.ImageValidatingPolicySpec{
			ValidationConfigurations: policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false)},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"},
						},
					},
				}},
			},
			MatchImageReferences: []policiesv1beta1.MatchImageReference{{Glob: "registry.example/*"}},
			Validations:          []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	provider, err := ivpolengine.NewProvider(evaluator.NewCompiler(nil), []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	namespaceResolver := func(s string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: s}}
	}
	engine := ivpolengine.NewEngine(provider, namespaceResolver, matching.NewMatcher(), nil, imageverifycache.DisabledImageVerifyCache(), config.NewDefaultConfiguration(false))
	h := New(engine, libs.NewFakeContextProvider(), nil, false, event.NewFake())
	request := admissionv1.AdmissionRequest{
		UID: "panic-test", Name: "web", Namespace: "default", Operation: admissionv1.Create,
		Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
		Resource: metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
		Object: runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Pod",
			"metadata":{"name":"web","namespace":"default"},
			"spec":{"containers":[{"name":"app","image":"` + image + `"}]}}`)},
	}
	request.RequestResource = &request.Resource

	response := h.ValidateClustered(context.Background(), logr.Discard(), handlers.AdmissionRequest{AdmissionRequest: request}, "", time.Now())
	require.False(t, response.Allowed)
	require.NotNil(t, response.Result)
	require.Contains(t, response.Result.Message, "injected panic while serving")
}
