package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type provenanceProtectionToggles struct {
	toggle.Toggles
	enabled bool
}

func (t provenanceProtectionToggles) ProtectManagedResources() bool { return t.enabled }

func TestServerGenerateProvenanceControllerIdentities(t *testing.T) {
	t.Parallel()
	const background = "system:serviceaccount:background-system:custom-background"
	for _, enabled := range []bool{false, true} {
		ctx := toggle.NewContext(context.Background(), provenanceProtectionToggles{Toggles: toggle.FromContext(context.Background()), enabled: enabled})
		router := buildTestServerWithContext(t, ctx, background)
		for _, path := range []string{"/validate/fail", "/validate/ignore", "/mutate/fail", "/gpol/sample"} {
			for _, test := range []struct {
				name, username string
				allowed        bool
				managed        bool
				managedAllowed bool
			}{
				{name: "admission controller", username: config.KyvernoUserName(config.KyvernoServiceAccountName()), allowed: true, managed: true, managedAllowed: true},
				{name: "external background controller on managed resource", username: background, allowed: true, managed: true},
				{name: "external background controller on ordinary resource", username: background, allowed: true, managedAllowed: true},
				{name: "unrelated installation account", username: config.KyvernoUserName("untrusted")},
				{name: "background name suffix", username: background + "-untrusted"},
			} {
				t.Run(fmt.Sprintf("protect=%t/%s/%s", enabled, path, test.name), func(t *testing.T) {
					t.Parallel()
					var labels map[string]string
					if test.managed {
						labels = map[string]string{"app.kubernetes.io/managed-by": "kyverno"}
					}
					object, err := json.Marshal(map[string]any{
						"apiVersion": "v1", "kind": "Secret",
						"metadata": map[string]any{
							"name": "target", "namespace": "tenant",
							"labels":      labels,
							"annotations": map[string]string{provenance.Annotation: "v1:controller-stamp"},
						},
					})
					require.NoError(t, err)
					request := &admissionv1.AdmissionRequest{
						UID: "provenance-request", Namespace: "tenant", Operation: admissionv1.Create,
						Kind: metav1.GroupVersionKind{Version: "v1", Kind: "Secret"}, Resource: metav1.GroupVersionResource{Version: "v1", Resource: "secrets"},
						UserInfo: authenticationv1.UserInfo{Username: test.username}, Object: runtime.RawExtension{Raw: object},
					}
					body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Request: request})
					require.NoError(t, err)
					httpRequest := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
					httpRequest.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httpRequest)
					require.Equal(t, http.StatusOK, response.Code)
					var review admissionv1.AdmissionReview
					require.NoError(t, json.Unmarshal(response.Body.Bytes(), &review))
					require.NotNil(t, review.Response)
					assert.Equal(t, test.allowed && (!enabled || test.managedAllowed), review.Response.Allowed)
				})
			}
		}
	}
}
