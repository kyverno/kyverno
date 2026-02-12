package utils

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testCaseStruct struct {
	name         string
	request      admissionv1.AdmissionRequest
	roles        []string
	clusterRoles []string
	gvk          schema.GroupVersionKind
	wantErr      bool
}

func TestPolicyContextBuilderBuild(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	builder := NewPolicyContextBuilder(cfg, jp)

	tests := []testCaseStruct{
		{
			name: "Basic Pod Creation Request",
			request: admissionv1.AdmissionRequest{
				UID:       "12345",
				Operation: admissionv1.Create,
				Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
				UserInfo:  authenticationv1.UserInfo{Username: "system:serviceaccount:default:test-user"},
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod"}}`)},
			},
			roles:        []string{"admin"},
			clusterRoles: []string{"cluster-admin"},
			gvk:          schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			wantErr:      false,
		},
		{
			name: "DryRun Request",
			request: admissionv1.AdmissionRequest{
				UID:       "67890",
				Operation: admissionv1.Update,
				Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				UserInfo:  authenticationv1.UserInfo{Username: "deployer"},
				DryRun:    boolPtr(true),
				Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deploy"}}`)},
			},
			roles:        []string{},
			clusterRoles: []string{},
			gvk:          schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := builder.Build(tt.request, tt.roles, tt.clusterRoles, tt.gvk)
			validateContext(t, got, err, tt)
		})
	}
}

func validateContext(t *testing.T, got *engine.PolicyContext, err error, tt testCaseStruct) {
	if (err != nil) != tt.wantErr {
		t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
		return
	}

	if tt.wantErr || got == nil {
		return
	}

	resource := got.NewResource()
	if resource.GroupVersionKind() != tt.gvk {
		t.Errorf("Build() GVK = %v, want %v", resource.GroupVersionKind(), tt.gvk)
	}

	admInfo := got.AdmissionInfo()
	if len(admInfo.Roles) != len(tt.roles) {
		t.Errorf("Build() Roles count = %d, want %d", len(admInfo.Roles), len(tt.roles))
	}
}

func boolPtr(b bool) *bool {
	return &b
}
