package validation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestManifestPolicyKeyURLUsesNativeVerifier(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = response.Write([]byte(ecdsaPub))
	}))
	defer server.Close()
	policyContext := buildContext(t, kyvernov1.Create, test_policy, signed_resource, "")
	var request admissionv1.AdmissionRequest
	require.NoError(t, json.Unmarshal([]byte(signed_adreq), &request))
	require.NoError(t, policyContext.JSONContext().AddRequest(request))
	policyContext.Policy().SetName("policy-key-url")
	verified, _, err := h.verifyManifest(context.Background(), logr.Discard(), policyContext, kyvernov1.Manifests{
		Attestors: []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{{Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: server.URL + "/key"}}}}},
	})
	require.NoError(t, err)
	require.True(t, verified)
	require.Positive(t, requests.Load())
}

func TestManifestPolicyBundleRepositoryUsesNativeVerifier(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.NotFound(response, request)
	}))
	defer server.Close()
	var resource unstructured.Unstructured
	require.NoError(t, json.Unmarshal([]byte(signed_resource), &resource))
	repository := strings.TrimPrefix(server.URL, "http://") + "/policy/bundle:latest"
	_, _, err := k8sVerifyResource(resource, kyvernov1.Attestor{
		Repository: repository, Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: ecdsaPub},
	}, &k8smanifest.VerifyResourceOption{AllowInsecure: true, DisableDryRun: true}, ".attestors[0]", "bundle-policy", 0, logr.Discard())
	require.Error(t, err)
	require.NotErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	require.NotContains(t, err.Error(), "connection blocked to network host")
	require.Positive(t, requests.Load())
}
