package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestWithProtection_NamespaceControllerDeletion(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Delete,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: namespaceControllerUsername,
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			OldObject: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.True(t, called, "Inner handler should be called for namespace controller deletion")
	assert.True(t, response.Allowed, "Namespace controller should be allowed to delete resources")
}

func TestWithProtection_KyvernoManagedResource_NonKyvernoUser(t *testing.T) {
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:user",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.False(t, response.Allowed, "Non-Kyverno user should not be allowed to modify Kyverno-managed resources")
	assert.NotNil(t, response.Result, "Result should contain error message")
	assert.Contains(t, response.Result.Message, "kyverno managed resource", "Error message should mention Kyverno managed resource")
}

func TestWithProtection_KyvernoManagedResource_KyvernoUser(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:kyverno:kyverno-service-account",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())

	assert.True(t, called, "Inner handler should be called for Kyverno user")
	assert.True(t, response.Allowed, "Kyverno service account should be allowed to modify Kyverno-managed resources")
}

func TestWithProtection_NonKyvernoManagedResource(t *testing.T) {
	called := false
	mockInner := AdmissionHandler(func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		called = true
		return admissionv1.AdmissionResponse{
			UID:     request.UID,
			Allowed: true,
		}
	})

	handler := mockInner.WithProtection(true)

	pod := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "my-app",
			},
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "test",
					"image": "nginx",
				},
			},
		},
	}

	podRaw, _ := json.Marshal(pod)

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: admissionv1.Update,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Resource: metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Namespace: "default",
			Name:      "test-pod",
			UserInfo: authenticationv1.UserInfo{
				Username: "system:serviceaccount:default:user",
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			RequestResource: &metav1.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods",
			},
			Object: runtime.RawExtension{
				Raw: podRaw,
			},
		},
	}

	response := handler(context.Background(), logr.Discard(), request, time.Now())
	assert.True(t, called, "Inner handler should be called for non-Kyverno managed resources")
	assert.True(t, response.Allowed, "Regular user should be allowed to modify non-Kyverno managed resources")
}

func TestWithGenerateLabelProtection(t *testing.T) {
	t.Parallel()
	const (
		user           = "system:serviceaccount:tenant:user"
		kyvernoUser    = "system:serviceaccount:kyverno:kyverno-background-controller"
		generatePolicy = common.GeneratePolicyLabel
		cloneSource    = common.GenerateTypeCloneSourceLabel
	)
	tests := []struct {
		name                    string
		operation               admissionv1.Operation
		username                string
		oldLabels               map[string]string
		newLabels               map[string]string
		protectManagedResources bool
		wantAllowed             bool
		wantError               string
	}{
		{
			name:        "allows ordinary labels on create",
			operation:   admissionv1.Create,
			username:    user,
			newLabels:   map[string]string{"app": "demo"},
			wantAllowed: true,
		},
		{
			name:      "denies generate labels on create",
			operation: admissionv1.Create,
			username:  user,
			newLabels: map[string]string{generatePolicy: "sync-secrets"},
		},
		{
			name:        "allows standalone Kyverno managed-by on create",
			operation:   admissionv1.Create,
			username:    user,
			newLabels:   map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			wantAllowed: true,
		},
		{
			name:        "allows clone source on create",
			operation:   admissionv1.Create,
			username:    user,
			newLabels:   map[string]string{cloneSource: ""},
			wantAllowed: true,
		},
		{
			name:        "allows restoring clone source with ordinary managed-by",
			operation:   admissionv1.Create,
			username:    user,
			newLabels:   map[string]string{cloneSource: "", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			wantAllowed: true,
		},
		{
			name:        "allows adding clone source on update",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{"app": "demo"},
			newLabels:   map[string]string{"app": "demo", cloneSource: ""},
			wantAllowed: true,
		},
		{
			name:        "allows changing clone source on update",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{cloneSource: ""},
			newLabels:   map[string]string{cloneSource: "restored"},
			wantAllowed: true,
		},
		{
			name:        "allows replacing user-owned clone source without marker",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{"app": "demo", cloneSource: ""},
			newLabels:   map[string]string{"app": "replaced"},
			wantAllowed: true,
		},
		{
			name:        "allows adding standalone Kyverno managed-by on update",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{"app": "demo"},
			newLabels:   map[string]string{"app": "demo", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			wantAllowed: true,
		},
		{
			name:        "allows removing standalone Kyverno managed-by on update",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			newLabels:   map[string]string{},
			wantAllowed: true,
		},
		{
			name:        "allows changing standalone Kyverno managed-by on update",
			operation:   admissionv1.Update,
			username:    user,
			oldLabels:   map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			newLabels:   map[string]string{kyverno.LabelAppManagedBy: "another-controller"},
			wantAllowed: true,
		},
		{
			name:                    "allows clone source replacement with managed-resource protection enabled",
			operation:               admissionv1.Update,
			username:                user,
			oldLabels:               map[string]string{cloneSource: ""},
			newLabels:               map[string]string{},
			protectManagedResources: true,
			wantAllowed:             true,
		},
		{
			name:                    "denies standalone Kyverno managed-by with managed-resource protection enabled",
			operation:               admissionv1.Create,
			username:                user,
			newLabels:               map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			protectManagedResources: true,
			wantError:               "kyverno managed resource can only be modified by kyverno",
		},
		{
			name:                    "denies removing standalone Kyverno managed-by with managed-resource protection enabled",
			operation:               admissionv1.Update,
			username:                user,
			oldLabels:               map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			newLabels:               map[string]string{},
			protectManagedResources: true,
			wantError:               "kyverno managed resource can only be modified by kyverno",
		},
		{
			name:      "denies restoring generated routing metadata beside clone source",
			operation: admissionv1.Create,
			username:  user,
			newLabels: map[string]string{cloneSource: "", common.GenerateTriggerUIDLabel: "trigger-uid"},
		},
		{
			name:      "denies reserved metadata in Kyverno namespace lookalike",
			operation: admissionv1.Create,
			username:  "system:serviceaccount:kyverno-tenant:user",
			newLabels: map[string]string{generatePolicy: "sync-secrets"},
		},
		{
			name:      "denies adding generate labels on update",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{"app": "demo"},
			newLabels: map[string]string{"app": "demo", generatePolicy: "sync-secrets"},
		},
		{
			name:      "denies changing generate labels on update",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "original"},
			newLabels: map[string]string{generatePolicy: "changed"},
		},
		{
			name:      "denies removing generate labels on update",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets"},
			newLabels: map[string]string{},
		},
		{
			name:      "denies adding Kyverno managed-by on update",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets"},
			newLabels: map[string]string{
				generatePolicy:            "sync-secrets",
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		{
			name:      "denies removing managed-by with unchanged generated routing",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			newLabels: map[string]string{generatePolicy: "sync-secrets"},
		},
		{
			name:      "denies changing managed-by with unchanged generated routing",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			newLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: "another-controller"},
		},
		{
			name:      "denies changes between other managed-by values with unchanged generated routing",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: "first-controller"},
			newLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: "second-controller"},
		},
		{
			name:      "denies adding empty managed-by with unchanged generated routing",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{generatePolicy: "sync-secrets"},
			newLabels: map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: ""},
		},
		{
			name:                    "allows namespace-controller deletion with managed-resource protection enabled",
			operation:               admissionv1.Delete,
			username:                namespaceControllerUsername,
			oldLabels:               map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			protectManagedResources: true,
			wantAllowed:             true,
		},
		{
			name:                    "denies user deletion with managed-resource protection enabled",
			operation:               admissionv1.Delete,
			username:                user,
			oldLabels:               map[string]string{generatePolicy: "sync-secrets", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
			protectManagedResources: true,
			wantError:               "kyverno managed resource can only be modified by kyverno",
		},
		{
			name:      "allows ordinary edits with unchanged generated metadata",
			operation: admissionv1.Update,
			username:  user,
			oldLabels: map[string]string{
				generatePolicy:            "sync-secrets",
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
			newLabels: map[string]string{
				"app":                     "edited",
				generatePolicy:            "sync-secrets",
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
			wantAllowed: true,
		},
		{
			name:        "allows Kyverno to set generated metadata",
			operation:   admissionv1.Create,
			username:    kyvernoUser,
			newLabels:   map[string]string{generatePolicy: "sync-secrets"},
			wantAllowed: true,
		},
		{
			name:        "allows generated resource deletion",
			operation:   admissionv1.Delete,
			username:    user,
			oldLabels:   map[string]string{generatePolicy: "sync-secrets"},
			wantAllowed: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			called := false
			inner := AdmissionHandler(func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse {
				called = true
				return admissionv1.AdmissionResponse{Allowed: true}
			})
			request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
				UID:       types.UID("test-uid"),
				Operation: test.operation,
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
				Namespace: "tenant",
				Name:      "test",
				UserInfo:  authenticationv1.UserInfo{Username: test.username},
			}}
			if test.operation != admissionv1.Delete {
				request.Object.Raw = resourceWithLabels(t, test.newLabels)
			}
			if test.operation != admissionv1.Create {
				request.OldObject.Raw = resourceWithLabels(t, test.oldLabels)
			}

			handler := inner.WithGenerateLabelProtection(kyvernoUser).WithProtection(test.protectManagedResources, kyvernoUser)
			response := handler(context.Background(), logr.Discard(), request, time.Now())
			assert.Equal(t, test.wantAllowed, response.Allowed)
			assert.Equal(t, test.wantAllowed, called)
			if !test.wantAllowed && assert.NotNil(t, response.Result) {
				if test.wantError == "" {
					test.wantError = "generate labels can only be set by Kyverno"
				}
				assert.Contains(t, response.Result.Message, test.wantError)
			}
		})
	}
}

func TestWithGenerateLabelProtection_ControllerIdentities(t *testing.T) {
	t.Parallel()
	const (
		admission        = "system:serviceaccount:kyverno:kyverno-admission-controller"
		background       = "system:serviceaccount:kyverno:kyverno-background-controller"
		customAdmission  = "system:serviceaccount:policy-system:custom-admission"
		customBackground = "system:serviceaccount:policy-system:custom-background"
	)
	tests := []struct {
		name        string
		username    string
		controllers []string
		wantAllowed bool
	}{
		{name: "configured admission controller", username: admission, controllers: []string{admission, background}, wantAllowed: true},
		{name: "configured background controller", username: background, controllers: []string{admission, background}, wantAllowed: true},
		{name: "unrelated account in installation namespace", username: "system:serviceaccount:kyverno:tenant", controllers: []string{admission, background}},
		{name: "default account in installation namespace", username: "system:serviceaccount:kyverno:default", controllers: []string{admission, background}},
		{name: "controller name in another namespace", username: "system:serviceaccount:tenant:kyverno-background-controller", controllers: []string{admission, background}},
		{name: "controller name suffix", username: background + "-tenant", controllers: []string{admission, background}},
		{name: "custom admission controller", username: customAdmission, controllers: []string{customAdmission, customBackground}, wantAllowed: true},
		{name: "custom background controller", username: customBackground, controllers: []string{customAdmission, customBackground}, wantAllowed: true},
		{name: "unconfigured default controller", username: background, controllers: []string{customAdmission, customBackground}},
		{name: "unrelated account in custom namespace", username: "system:serviceaccount:policy-system:tenant", controllers: []string{customAdmission, customBackground}},
		{name: "missing configuration", username: background},
		{name: "empty controller name", username: "", controllers: []string{""}},
	}
	for _, test := range tests {
		for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
			t.Run(test.name+"/"+string(operation), func(t *testing.T) {
				t.Parallel()
				called := false
				inner := AdmissionHandler(func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse {
					called = true
					return admissionv1.AdmissionResponse{Allowed: true}
				})
				request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("controller-identity"),
					Operation: operation,
					Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
					UserInfo:  authenticationv1.UserInfo{Username: test.username},
					Object:    runtime.RawExtension{Raw: resourceWithLabels(t, map[string]string{common.GeneratePolicyLabel: "new-policy"})},
				}}
				if operation == admissionv1.Update {
					request.OldObject.Raw = resourceWithLabels(t, map[string]string{common.GeneratePolicyLabel: "old-policy"})
				}
				response := inner.WithGenerateLabelProtection(test.controllers...)(context.Background(), logr.Discard(), request, time.Now())
				assert.Equal(t, test.wantAllowed, response.Allowed)
				assert.Equal(t, test.wantAllowed, called)
				if !test.wantAllowed && assert.NotNil(t, response.Result) {
					assert.Contains(t, response.Result.Message, "generate labels can only be set by Kyverno")
				}

				// Ordinary metadata updates remain available to every caller, even
				// when the caller is not a configured controller.
				request.Operation = admissionv1.Update
				request.OldObject = request.Object
				request.Object.Raw = resourceWithLabels(t, map[string]string{common.GeneratePolicyLabel: "new-policy", "app": "edited"})
				called = false
				response = inner.WithGenerateLabelProtection(test.controllers...)(context.Background(), logr.Discard(), request, time.Now())
				assert.True(t, response.Allowed)
				assert.True(t, called)
			})
		}
	}
}

func TestWithProtection_ConfiguredControllerIdentities(t *testing.T) {
	t.Parallel()
	const controller = "system:serviceaccount:external-controllers:custom-background"
	tests := []struct {
		name        string
		username    string
		controllers []string
		trusted     bool
	}{
		{name: "configured external controller", username: controller, controllers: []string{controller}, trusted: true},
		{name: "installation namespace compatibility", username: kyvernoUsernamePrefix + "existing-account", trusted: true},
		{name: "unconfigured external controller", username: controller},
		{name: "different external account", username: "system:serviceaccount:external-controllers:other", controllers: []string{controller}},
		{name: "controller name suffix", username: controller + "-other", controllers: []string{controller}},
		{name: "empty controller name", controllers: []string{""}},
	}
	for _, test := range tests {
		for _, enabled := range []bool{false, true} {
			for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update, admissionv1.Delete} {
				t.Run(fmt.Sprintf("%s/enabled=%t/%s", test.name, enabled, operation), func(t *testing.T) {
					t.Parallel()
					called := false
					inner := AdmissionHandler(func(_ context.Context, _ logr.Logger, request AdmissionRequest, _ time.Time) AdmissionResponse {
						called = true
						return admissionv1.AdmissionResponse{UID: request.UID, Allowed: true}
					})
					request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
						UID: "managed-controller-identity", Operation: operation,
						Kind:     metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
						UserInfo: authenticationv1.UserInfo{Username: test.username},
					}}
					resource := runtime.RawExtension{Raw: resourceWithLabels(t, map[string]string{
						kyverno.LabelAppManagedBy:  kyverno.ValueKyvernoApp,
						common.GeneratePolicyLabel: "policy",
					})}
					if operation != admissionv1.Delete {
						request.Object = resource
					}
					if operation != admissionv1.Create {
						request.OldObject = resource
					}
					response := inner.WithProtection(enabled, test.controllers...)(context.Background(), logr.Discard(), request, time.Now())
					allowed := !enabled || test.trusted
					assert.Equal(t, allowed, response.Allowed)
					assert.Equal(t, allowed, called)
					assert.Equal(t, request.UID, response.UID)
					if !allowed {
						require.NotNil(t, response.Result)
						assert.Contains(t, response.Result.Message, "kyverno managed resource can only be modified by kyverno")
					}
				})
			}
		}
	}
}

func TestWithProtection_DisabledDoesNotDecodeResources(t *testing.T) {
	t.Parallel()
	called := false
	request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
		UID: "no-protection", Operation: admissionv1.Update,
		Object:    runtime.RawExtension{Raw: []byte("invalid new object")},
		OldObject: runtime.RawExtension{Raw: []byte("invalid old object")},
	}}
	inner := AdmissionHandler(func(_ context.Context, _ logr.Logger, received AdmissionRequest, _ time.Time) AdmissionResponse {
		called = true
		assert.Equal(t, request, received)
		return admissionv1.AdmissionResponse{UID: received.UID, Allowed: true}
	})
	response := inner.WithProtection(false)(context.Background(), logr.Discard(), request, time.Now())
	assert.True(t, called)
	assert.True(t, response.Allowed)
}

func TestWithGenerateLabelProtection_InvalidResources(t *testing.T) {
	t.Parallel()
	for _, old := range []bool{false, true} {
		t.Run(fmt.Sprintf("old=%t", old), func(t *testing.T) {
			t.Parallel()
			called := false
			inner := AdmissionHandler(func(context.Context, logr.Logger, AdmissionRequest, time.Time) AdmissionResponse {
				called = true
				return admissionv1.AdmissionResponse{Allowed: true}
			})
			resource := runtime.RawExtension{Raw: resourceWithLabels(t, map[string]string{common.GeneratePolicyLabel: "policy"})}
			request := AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
				UID: "invalid-resource", Operation: admissionv1.Update,
				Object: resource, OldObject: resource,
			}}
			if old {
				request.OldObject.Raw = []byte("invalid object")
			} else {
				request.Object.Raw = []byte("invalid object")
			}
			response := inner.WithGenerateLabelProtection()(context.Background(), logr.Discard(), request, time.Now())
			assert.False(t, called)
			assert.False(t, response.Allowed)
			assert.Equal(t, request.UID, response.UID)
			require.NotNil(t, response.Result)
			assert.Contains(t, response.Result.Message, "failed to convert")
		})
	}
}

func resourceWithLabels(t *testing.T, labels map[string]string) []byte {
	t.Helper()
	resource := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "tenant",
			"labels":    labels,
		},
	}
	raw, err := json.Marshal(resource)
	assert.NoError(t, err)
	return raw
}
