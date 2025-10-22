package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionStatus_SetReadyByCondition_True(t *testing.T) {
	var status ConditionStatus
	status.SetReadyByCondition(PolicyConditionTypeWebhookConfigured, metav1.ConditionTrue, "dummy")
	got := meta.FindStatusCondition(status.Conditions, string(PolicyConditionTypeWebhookConfigured))
	assert.NotNil(t, got)
	assert.Equal(t, string(PolicyConditionTypeWebhookConfigured), got.Type)
	assert.Equal(t, metav1.ConditionTrue, got.Status)
	assert.Equal(t, "Succeeded", got.Reason)
	assert.Equal(t, "dummy", got.Message)
}

func TestConditionStatus_SetReadyByCondition_False(t *testing.T) {
	var status ConditionStatus
	status.SetReadyByCondition(PolicyConditionTypeWebhookConfigured, metav1.ConditionFalse, "dummy")
	got := meta.FindStatusCondition(status.Conditions, string(PolicyConditionTypeWebhookConfigured))
	assert.NotNil(t, got)
	assert.Equal(t, string(PolicyConditionTypeWebhookConfigured), got.Type)
	assert.Equal(t, metav1.ConditionFalse, got.Status)
	assert.Equal(t, "Failed", got.Reason)
	assert.Equal(t, "dummy", got.Message)
}
