package ivpol

import (
	"context"
	"crypto/tls"
	"fmt"
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

type diagnosticTransport func(*http.Request) (*http.Response, error)

func (f diagnosticTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// This runs real registry loading, Cosign, CEL, the engine and the webhook. Only
// HTTP transport is replaced: no sockets or external Sigstore services are used.
func TestCosignDiagnosticsAdmission(t *testing.T) {
	// DefaultTransport is process-global, so this test must not run in parallel.
	oldTransport := regcreds.DefaultTransport
	transport := &http.Transport{TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}}
	regcreds.DefaultTransport = transport
	t.Cleanup(func() { regcreds.DefaultTransport = oldTransport; transport.CloseIdleConnections() })
	registryHandler := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	var verificationError error
	var verificationRequests int
	transport.RegisterProtocol("https", diagnosticTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, ".sig") || strings.HasSuffix(r.URL.Path, ".att") {
			verificationRequests++
			if verificationError != nil {
				return nil, verificationError
			}
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
	const image = "registry.example/image:tag"
	ref, err := name.ParseReference(image)
	require.NoError(t, err)
	require.NoError(t, remote.Write(ref, empty.Image, remote.WithTransport(transport)))
	const key = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPEDZl3iOJwr77T2bS9vgonwzERmG
PKd/xnmHKfvkbLquVC6NnH8dgPVq8p0H45H2H9CqzqGv+rn99xAWGLE30A==
-----END PUBLIC KEY-----`
	for _, attestation := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			err  error
			want string
		}{
			{"canceled", context.Canceled, "context canceled"},
			{"deadline", context.DeadlineExceeded, "context deadline exceeded"},
			{"missing signature", nil, ""},
		} {
			t.Run(fmt.Sprintf("%s/attestation=%t", tc.name, attestation), func(t *testing.T) {
				verificationError = tc.err
				verificationRequests = 0
				call := `verifyImageSignatures(image, [attestors.key])`
				if attestation {
					call = `verifyAttestationSignatures(image, attestations.proof, [attestors.key])`
				}
				expression := fmt.Sprintf(`images.containers.map(image, %s).all(e, e > 0) && images.initContainers.map(image, %s).all(e, e > 0) && images.ephemeralContainers.map(image, %s).all(e, e > 0)`, call, call, call)
				policy := &policiesv1beta1.ImageValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image-signatures"},
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						Credentials:              &policiesv1beta1.Credentials{},
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
						Attestors: []policiesv1beta1.Attestor{{
							Name: "key",
							Cosign: &policiesv1beta1.Cosign{
								Key: &policiesv1beta1.Key{Data: key}, CTLog: &policiesv1beta1.CTLog{InsecureIgnoreTlog: true},
							},
						}},
						Attestations: []policiesv1beta1.Attestation{{Name: "proof", InToto: &policiesv1beta1.InToto{Type: "https://example.com/proof"}}},
						Validations:  []admissionregistrationv1.Validation{{Expression: expression, Message: "Image signature verification failed"}},
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
					UID: "diagnostic-test", Name: "pod", Namespace: "default", Operation: admissionv1.Create,
					Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
					Resource: metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
					Object: runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{"apiVersion":"v1","kind":"Pod",
						"metadata":{"name":"pod","namespace":"default"},
						"spec":{"containers":[{"name":"main","image":%q}]}}`, image))},
				}
				request.RequestResource = &request.Resource
				response := h.ValidateClustered(context.Background(), logr.Discard(), handlers.AdmissionRequest{AdmissionRequest: request}, "", time.Now())
				require.False(t, response.Allowed)
				require.NotNil(t, response.Result)
				require.Greater(t, verificationRequests, 0, "the failure must occur during signature verification, not image loading: %s", response.Result.Message)
				require.Contains(t, response.Result.Message, "Policy verify-image-signatures failed: Image signature verification failed; verification details:")
				require.Contains(t, response.Result.Message, "failed to verify cosign signatures:")
				require.Contains(t, response.Result.Message, `attestor "key"`)
				if tc.want != "" {
					require.Contains(t, response.Result.Message, tc.want)
				}
			})
		}
	}
}
