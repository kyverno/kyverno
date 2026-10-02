package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/webhooks/auth"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type recordingVerifier struct {
	token, audience, group string
	err                    error
}

func (v *recordingVerifier) Verify(_ context.Context, token, audience, group string) error {
	v.token, v.audience, v.group = token, audience, group
	return v.err
}

func TestAdmissionAuthenticationBoundary(t *testing.T) {
	called := 0
	patchType := admissionv1.PatchTypeJSONPatch
	inner := AdmissionHandler(func(_ context.Context, _ logr.Logger, request AdmissionRequest, _ time.Time) AdmissionResponse {
		called++
		return AdmissionResponse{UID: request.UID, Allowed: true, PatchType: &patchType, Patch: []byte(`[{"op":"add","path":"/metadata/labels/test","value":"yes"}]`)}
	})
	verifier := &recordingVerifier{}
	receiver := auth.NewReceiver(verifier, "", 443)
	review, err := json.Marshal(admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{
		UID: types.UID("auth-test"), Resource: metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}})
	require.NoError(t, err)
	send := func(authorization string, enabled bool) admissionv1.AdmissionReview {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/validate/fail", bytes.NewReader(review))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", authorization)
		request.Host = "attacker.example"
		response := httptest.NewRecorder()
		if enabled {
			inner.withAdmission(logr.Discard(), receiver)(response, request)
		} else {
			inner.withAdmission(logr.Discard())(response, request)
		}
		require.Equal(t, http.StatusOK, response.Code)
		var output admissionv1.AdmissionReview
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &output))
		return output
	}
	denied := send("", true)
	require.False(t, denied.Response.Allowed)
	require.Equal(t, types.UID("auth-test"), denied.Response.UID)
	require.Equal(t, int32(http.StatusUnauthorized), denied.Response.Result.Code)
	require.Zero(t, called)
	verifier.err = errors.New("sensitive verification detail")
	denied = send("Bearer token", true)
	require.NotContains(t, denied.Response.Result.Message, "sensitive")
	require.Zero(t, called)
	require.Equal(t, "token", verifier.token)
	require.Equal(t, "apps", verifier.group)
	require.Contains(t, verifier.audience, "/validate/fail")
	verifier.err = nil
	allowed := send("Bearer token", true)
	require.True(t, allowed.Response.Allowed)
	require.Equal(t, patchType, *allowed.Response.PatchType)
	require.Equal(t, 1, called)
	legacy := send("", false)
	require.True(t, legacy.Response.Allowed)
	require.Equal(t, 2, called)
}
