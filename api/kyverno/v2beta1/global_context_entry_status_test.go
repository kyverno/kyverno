package v2beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGlobalContextEntryStatus_SetReady_True(t *testing.T) {
	status := &GlobalContextEntryStatus{}

	assert.False(t, status.IsReady())

	status.SetReady(true, "Successfully refreshed entry")

	assert.True(t, status.IsReady())
	assert.Nil(t, status.Ready)
	assert.Len(t, status.Conditions, 1)

	cond := status.Conditions[0]
	assert.Equal(t, GlobalContextEntryConditionReady, cond.Type)
	assert.Equal(t, metav1.ConditionTrue, cond.Status)
	assert.Equal(t, GlobalContextEntryReasonSucceeded, cond.Reason)
	assert.Equal(t, "Successfully refreshed entry", cond.Message)
}

func TestGlobalContextEntryStatus_SetReady_False(t *testing.T) {
	status := &GlobalContextEntryStatus{}

	status.SetReady(true, "Ready")
	assert.True(t, status.IsReady())

	status.SetReady(false, "API call timed out")

	assert.False(t, status.IsReady())
	assert.Nil(t, status.Ready)
	assert.Len(t, status.Conditions, 1)

	cond := status.Conditions[0]
	assert.Equal(t, GlobalContextEntryConditionReady, cond.Type)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, GlobalContextEntryReasonFailed, cond.Reason)
	assert.Equal(t, "API call timed out", cond.Message)
}

func TestGlobalContextEntryStatus_UpdateRefreshTime(t *testing.T) {
	status := &GlobalContextEntryStatus{}
	assert.True(t, status.LastRefreshTime.IsZero())

	status.UpdateRefreshTime()
	assert.False(t, status.LastRefreshTime.IsZero())
}
