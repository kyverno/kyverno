package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestServerControllerManagedResourceCompatibility(t *testing.T) {
	t.Parallel()
	ctx := toggle.NewContext(context.Background(), serverProtectionToggles{Toggles: toggle.FromContext(context.Background()), enabled: true})
	// Cleanup and reports accounts are deliberately absent from the routing
	// controllers. Their generic managed-resource access must remain separate.
	router := buildTestServerWithContext(t, ctx, "system:serviceaccount:external-controllers:custom-background")
	managed := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"target","labels":{"app.kubernetes.io/managed-by":"kyverno"}}}`
	edited := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"target","labels":{"app.kubernetes.io/managed-by":"kyverno"}},"data":{"value":"ZWRpdGVk"}}`
	associated := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"target","labels":{"app.kubernetes.io/managed-by":"kyverno","generate.kyverno.io/policy-name":"policy"}}}`
	retargeted := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"target","labels":{"app.kubernetes.io/managed-by":"kyverno","generate.kyverno.io/policy-name":"other-policy"}}}`
	for _, caller := range []struct {
		name, username string
		managedAllowed bool
	}{
		{name: "cleanup controller", username: config.KyvernoUserName("kyverno-cleanup-controller"), managedAllowed: true},
		{name: "reports controller", username: config.KyvernoUserName("kyverno-reports-controller"), managedAllowed: true},
		{name: "custom cleanup account", username: config.KyvernoUserName("custom-cleanup"), managedAllowed: true},
		{name: "custom reports account", username: config.KyvernoUserName("custom-reports"), managedAllowed: true},
		{name: "unrelated installation account", username: config.KyvernoUserName("unrelated"), managedAllowed: true},
		{name: "controller name in another namespace", username: "system:serviceaccount:other:kyverno-cleanup-controller"},
	} {
		for _, requestCase := range []struct {
			name, route, object, oldObject string
			operation                      admissionv1.Operation
			allowed                        bool
		}{
			{name: "update managed content", route: config.ValidatingWebhookServicePath, operation: admissionv1.Update, object: edited, oldObject: managed, allowed: caller.managedAllowed},
			{name: "delete managed target", route: config.ValidatingWebhookServicePath, operation: admissionv1.Delete, oldObject: associated, allowed: caller.managedAllowed},
			{name: "create routing metadata", route: config.GenerationLabelProtectionWebhookServicePath, operation: admissionv1.Create, object: associated},
			{name: "change routing metadata", route: config.GenerationLabelProtectionWebhookServicePath, operation: admissionv1.Update, object: retargeted, oldObject: associated},
			{name: "preserve routing metadata", route: config.GenerationLabelProtectionWebhookServicePath, operation: admissionv1.Update, object: associated, oldObject: associated, allowed: true},
		} {
			t.Run(caller.name+"/"+requestCase.name, func(t *testing.T) {
				t.Parallel()
				request := &admissionv1.AdmissionRequest{
					UID: "controller-compatibility", Namespace: "tenant", Name: "target", Operation: requestCase.operation,
					Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
					Resource: metav1.GroupVersionResource{Version: "v1", Resource: "secrets"},
					UserInfo: authenticationv1.UserInfo{Username: caller.username},
				}
				if requestCase.object != "" {
					request.Object = runtime.RawExtension{Raw: []byte(requestCase.object)}
				}
				if requestCase.oldObject != "" {
					request.OldObject = runtime.RawExtension{Raw: []byte(requestCase.oldObject)}
				}
				body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Request: request})
				require.NoError(t, err)
				httpRequest := httptest.NewRequest(http.MethodPost, requestCase.route, bytes.NewReader(body))
				httpRequest.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httpRequest)
				require.Equal(t, http.StatusOK, response.Code)
				var review admissionv1.AdmissionReview
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &review))
				require.NotNil(t, review.Response)
				assert.Equal(t, request.UID, review.Response.UID)
				assert.Equal(t, requestCase.allowed, review.Response.Allowed)
			})
		}
	}
}
