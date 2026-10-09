package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWithProtection_GenerateProvenanceAnnotation(t *testing.T) {
	t.Parallel()
	const user = "system:serviceaccount:tenant:user"
	for _, protectManagedResources := range []bool{false, true} {
		for _, test := range []struct {
			name        string
			operation   admissionv1.Operation
			username    string
			old         map[string]string
			new         map[string]string
			wantAllowed bool
		}{
			{name: "denies stamp on create", operation: admissionv1.Create, new: map[string]string{provenance.Annotation: "v1:copied"}},
			{name: "denies empty stamp on create", operation: admissionv1.Create, new: map[string]string{provenance.Annotation: ""}},
			{name: "denies adding stamp", operation: admissionv1.Update, new: map[string]string{provenance.Annotation: "v1:copied"}},
			{name: "denies changing stamp", operation: admissionv1.Update, old: map[string]string{provenance.Annotation: "v1:original"}, new: map[string]string{provenance.Annotation: "v1:copied"}},
			{name: "denies removing stamp", operation: admissionv1.Update, old: map[string]string{provenance.Annotation: "v1:original"}},
			{name: "denies removing empty stamp", operation: admissionv1.Update, old: map[string]string{provenance.Annotation: ""}},
			{name: "allows unchanged stamp", operation: admissionv1.Update, old: map[string]string{provenance.Annotation: "v1:original"}, new: map[string]string{provenance.Annotation: "v1:original"}, wantAllowed: true},
			{name: "allows ordinary annotation edits", operation: admissionv1.Update, old: map[string]string{provenance.Annotation: "v1:original", "example.com/key": "old"}, new: map[string]string{provenance.Annotation: "v1:original", "example.com/key": "new"}, wantAllowed: true},
			{name: "allows ordinary annotations on create", operation: admissionv1.Create, new: map[string]string{"example.com/key": "value"}, wantAllowed: true},
			{name: "allows Kyverno stamp", operation: admissionv1.Create, username: "system:serviceaccount:kyverno:kyverno-background-controller", new: map[string]string{provenance.Annotation: "v1:stamp"}, wantAllowed: true},
			{name: "allows Kyverno stamp rotation", operation: admissionv1.Update, username: "system:serviceaccount:kyverno:kyverno-background-controller", old: map[string]string{provenance.Annotation: "v1:old"}, new: map[string]string{provenance.Annotation: "v1:new"}, wantAllowed: true},
			{name: "allows deletion with unchanged old stamp", operation: admissionv1.Delete, old: map[string]string{provenance.Annotation: "v1:original"}, wantAllowed: true},
		} {
			t.Run(fmt.Sprintf("protect=%t/%s", protectManagedResources, test.name), func(t *testing.T) {
				t.Parallel()
				called := false
				inner := AdmissionHandler(func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse {
					called = true
					return admissionv1.AdmissionResponse{Allowed: true}
				})
				username := test.username
				if username == "" {
					username = user
				}
				request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
					UID: "test-uid", Operation: test.operation,
					Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
					Namespace: "tenant", Name: "test", UserInfo: authenticationv1.UserInfo{Username: username},
				}}
				if test.operation != admissionv1.Delete {
					request.Object.Raw = resourceWithAnnotations(t, test.new)
				}
				if test.operation != admissionv1.Create {
					request.OldObject.Raw = resourceWithAnnotations(t, test.old)
				}
				response := inner.WithProtection(protectManagedResources, "system:serviceaccount:kyverno:kyverno-background-controller")(context.Background(), logr.Discard(), request, time.Now())
				assert.Equal(t, test.wantAllowed, response.Allowed)
				assert.Equal(t, test.wantAllowed, called)
				if !test.wantAllowed && assert.NotNil(t, response.Result) {
					assert.Contains(t, response.Result.Message, "Kyverno generate provenance can only be set by Kyverno")
				}
			})
		}
	}
}

func TestWithProtection_GenerateProvenanceControllerIdentities(t *testing.T) {
	t.Parallel()
	const (
		admission        = "system:serviceaccount:kyverno:kyverno-admission-controller"
		background       = "system:serviceaccount:kyverno:kyverno-background-controller"
		customAdmission  = "system:serviceaccount:policy-system:custom-admission"
		customBackground = "system:serviceaccount:background-system:custom-background"
	)
	tests := []struct {
		name        string
		username    string
		controllers []string
		wantAllowed bool
	}{
		{name: "configured admission", username: admission, controllers: []string{admission, background}, wantAllowed: true},
		{name: "configured background", username: background, controllers: []string{admission, background}, wantAllowed: true},
		{name: "unrelated installation account", username: "system:serviceaccount:kyverno:untrusted", controllers: []string{admission, background}},
		{name: "default installation account", username: "system:serviceaccount:kyverno:default", controllers: []string{admission, background}},
		{name: "controller name in another namespace", username: "system:serviceaccount:tenant:kyverno-background-controller", controllers: []string{admission, background}},
		{name: "controller name suffix", username: background + "-untrusted", controllers: []string{admission, background}},
		{name: "custom admission", username: customAdmission, controllers: []string{customAdmission, customBackground}, wantAllowed: true},
		{name: "custom background in another namespace", username: customBackground, controllers: []string{customAdmission, customBackground}, wantAllowed: true},
		{name: "unconfigured default controller", username: background, controllers: []string{customAdmission, customBackground}},
		{name: "unrelated custom namespace account", username: "system:serviceaccount:policy-system:untrusted", controllers: []string{customAdmission, customBackground}},
		{name: "missing configuration", username: background},
		{name: "empty configured identity", controllers: []string{""}},
	}
	resource := func(stamp string) []byte {
		data, err := json.Marshal(map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{
				"name": "target", "namespace": "tenant",
				"labels":      map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
				"annotations": map[string]string{provenance.Annotation: stamp},
			},
		})
		assert.NoError(t, err)
		return data
	}
	for _, test := range tests {
		for _, enabled := range []bool{false, true} {
			for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
				t.Run(fmt.Sprintf("%s/protect=%t/%s", test.name, enabled, operation), func(t *testing.T) {
					t.Parallel()
					called := false
					inner := AdmissionHandler(func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse {
						called = true
						return admissionv1.AdmissionResponse{Allowed: true}
					})
					request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
						UID: "provenance-identity", Operation: operation,
						Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
						UserInfo: authenticationv1.UserInfo{Username: test.username},
					}}
					request.Object.Raw = resource("v1:new")
					if operation == admissionv1.Update {
						request.OldObject.Raw = resource("v1:old")
					}
					response := inner.WithProtection(enabled, test.controllers...)(context.Background(), logr.Discard(), request, time.Now())
					assert.Equal(t, test.wantAllowed, response.Allowed)
					assert.Equal(t, test.wantAllowed, called)
					if !test.wantAllowed && assert.NotNil(t, response.Result) {
						assert.Contains(t, response.Result.Message, "can only be")
					}
				})
			}
		}
	}
}

func resourceWithAnnotations(t *testing.T, annotations map[string]string) []byte {
	t.Helper()
	resource := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":        "test",
			"namespace":   "tenant",
			"annotations": annotations,
		},
	}
	raw, err := json.Marshal(resource)
	assert.NoError(t, err)
	return raw
}
