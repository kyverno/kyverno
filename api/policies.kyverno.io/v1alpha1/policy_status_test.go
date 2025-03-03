package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPolicyStatus_IsReady(t *testing.T) {
	tests := []struct {
		name   string
		status PolicyStatus
		want   bool
	}{{
		name:   "nil",
		status: PolicyStatus{},
		want:   false,
	}, {
		name: "true",
		status: PolicyStatus{
			Ready: ptr.To(true),
		},
		want: true,
	}, {
		name: "false",
		status: PolicyStatus{
			Ready: ptr.To(false),
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsReady()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPolicyStatus_SetReadyByCondition_True(t *testing.T) {
	var status PolicyStatus
	status.SetReadyByCondition(PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "dummy")
	got := meta.FindStatusCondition(status.Conditions, string(PolicyConditionTypeWebhookConfigured))
	assert.NotNil(t, got)
	assert.Equal(t, string(PolicyConditionTypeWebhookConfigured), got.Type)
	assert.Equal(t, metav1.ConditionTrue, got.Status)
	assert.Equal(t, "Succeeded", got.Reason)
	assert.Equal(t, "dummy", got.Message)
}

func TestPolicyStatus_SetReadyByCondition_False(t *testing.T) {
	var status PolicyStatus
	status.SetReadyByCondition(PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "dummy")
	got := meta.FindStatusCondition(status.Conditions, string(PolicyConditionTypeWebhookConfigured))
	assert.NotNil(t, got)
	assert.Equal(t, string(PolicyConditionTypeWebhookConfigured), got.Type)
	assert.Equal(t, metav1.ConditionFalse, got.Status)
	assert.Equal(t, "Failed", got.Reason)
	assert.Equal(t, "dummy", got.Message)
}
