package deleting

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/utils/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_DeletingPolicy_WithFakeClient tests policy with deletion timestamp
func Test_DeletingPolicy_WithFakeClient(t *testing.T) {
	now := metav1.Now()
	pol := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting-test",
			Namespace:         "default",
			DeletionTimestamp: &now,
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "* * * * *",
		},
	}

	// NEW: Using testutil helper (drop-in replacement)
	fakeClient := testutil.NewKyvernoFakeClient(pol)

	ctx := context.Background()
	retrieved, err := fakeClient.KyvernoV2().CleanupPolicies("default").Get(ctx, "deleting-test", metav1.GetOptions{})

	require.NoError(t, err)
	assert.NotNil(t, retrieved.DeletionTimestamp)
	assert.Equal(t, "deleting-test", retrieved.Name)
}

// Test_DeletingPolicy_Finalizers tests finalizer handling
func Test_DeletingPolicy_Finalizers(t *testing.T) {
	pol := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "finalizer-test",
			Namespace:  "default",
			Finalizers: []string{"kyverno.io/cleanup"},
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "* * * * *",
		},
	}

	fakeClient := testutil.NewKyvernoFakeClient(pol)
	ctx := context.Background()

	retrieved, err := fakeClient.KyvernoV2().CleanupPolicies("default").Get(ctx, "finalizer-test", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Contains(t, retrieved.Finalizers, "kyverno.io/cleanup")

	// Remove finalizer
	retrieved.Finalizers = []string{}
	updated, err := fakeClient.KyvernoV2().CleanupPolicies("default").Update(ctx, retrieved, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Empty(t, updated.Finalizers)
}
