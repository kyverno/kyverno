package api

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRuleResponse_String(t *testing.T) {
	type fields struct {
		Name                           string
		Type                           RuleType
		Message                        string
		Patches                        [][]byte
		GeneratedResource              unstructured.Unstructured
		Status                         RuleStatus
		Stats                          ExecutionStats
		PatchedTarget                  *unstructured.Unstructured
		PatchedTargetSubresourceName   string
		PatchedTargetParentResourceGVR metav1.GroupVersionResource
		PodSecurityChecks              *PodSecurityChecks
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{
			Name:    "test-mutation",
			Type:    Mutation,
			Message: "message",
		},
		want: "rule test-mutation (Mutation): message",
	}, {
		fields: fields{
			Name:    "test-validation",
			Type:    Validation,
			Message: "message",
		},
		want: "rule test-validation (Validation): message",
	}, {
		fields: fields{
			Name:    "test-generation",
			Type:    Generation,
			Message: "message",
		},
		want: "rule test-generation (Generation): message",
	}, {
		fields: fields{
			Name:    "test-image-verify",
			Type:    ImageVerify,
			Message: "message",
		},
		want: "rule test-image-verify (ImageVerify): message",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := RuleResponse{
				Name:                           tt.fields.Name,
				Type:                           tt.fields.Type,
				Message:                        tt.fields.Message,
				Patches:                        tt.fields.Patches,
				GeneratedResource:              tt.fields.GeneratedResource,
				Status:                         tt.fields.Status,
				Stats:                          tt.fields.Stats,
				PatchedTarget:                  tt.fields.PatchedTarget,
				PatchedTargetSubresourceName:   tt.fields.PatchedTargetSubresourceName,
				PatchedTargetParentResourceGVR: tt.fields.PatchedTargetParentResourceGVR,
				PodSecurityChecks:              tt.fields.PodSecurityChecks,
			}
			if got := rr.String(); got != tt.want {
				t.Errorf("RuleResponse.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleResponse_HasStatus(t *testing.T) {
	type fields struct {
		Name                           string
		Type                           RuleType
		Message                        string
		Patches                        [][]byte
		GeneratedResource              unstructured.Unstructured
		Status                         RuleStatus
		Stats                          ExecutionStats
		PatchedTarget                  *unstructured.Unstructured
		PatchedTargetSubresourceName   string
		PatchedTargetParentResourceGVR metav1.GroupVersionResource
		PodSecurityChecks              *PodSecurityChecks
	}
	type args struct {
		status []RuleStatus
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{{
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusError},
		},
		want: false,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{RuleStatusError, RuleStatusPass, RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		args: args{
			status: []RuleStatus{},
		},
		want: false,
	}, {
		fields: fields{
			Status: RuleStatusFail,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RuleResponse{
				Name:                           tt.fields.Name,
				Type:                           tt.fields.Type,
				Message:                        tt.fields.Message,
				Patches:                        tt.fields.Patches,
				GeneratedResource:              tt.fields.GeneratedResource,
				Status:                         tt.fields.Status,
				Stats:                          tt.fields.Stats,
				PatchedTarget:                  tt.fields.PatchedTarget,
				PatchedTargetSubresourceName:   tt.fields.PatchedTargetSubresourceName,
				PatchedTargetParentResourceGVR: tt.fields.PatchedTargetParentResourceGVR,
				PodSecurityChecks:              tt.fields.PodSecurityChecks,
			}
			if got := r.HasStatus(tt.args.status...); got != tt.want {
				t.Errorf("RuleResponse.HasStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
