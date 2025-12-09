package cleanup

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/utils/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_CleanupPolicy_WithFakeClient demonstrates migration to new fake client
func Test_CleanupPolicy_WithFakeClient(t *testing.T) {
	pol := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default",
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "* * * * *",
		},
	}

	// OLD: fakeClient := versionedfake.NewSimpleClientset(pol.DeepCopy())
	// NEW: Use testutil helper
	fakeClient := testutil.NewKyvernoFakeClient(pol.DeepCopy())

	// Test we can retrieve it
	ctx := context.Background()
	retrieved, err := fakeClient.KyvernoV2().CleanupPolicies("default").Get(ctx, "test-policy", metav1.GetOptions{})

	require.NoError(t, err)
	assert.Equal(t, "test-policy", retrieved.Name)
	assert.Equal(t, "* * * * *", retrieved.Spec.Schedule)
}

// Test_CleanupPolicy_Lifecycle tests CRUD operations with fake client
func Test_CleanupPolicy_Lifecycle(t *testing.T) {
	ctx := context.Background()
	fakeClient := testutil.NewKyvernoFakeClient()

	// Create
	pol := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lifecycle-test",
			Namespace: "default",
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "*/5 * * * *",
		},
	}

	created, err := fakeClient.KyvernoV2().CleanupPolicies("default").Create(ctx, pol, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-test", created.Name)

	// Update
	created.Spec.Schedule = "*/10 * * * *"
	updated, err := fakeClient.KyvernoV2().CleanupPolicies("default").Update(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "*/10 * * * *", updated.Spec.Schedule)

	// List
	list, err := fakeClient.KyvernoV2().CleanupPolicies("default").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, list.Items, 1)

	// Delete
	err = fakeClient.KyvernoV2().CleanupPolicies("default").Delete(ctx, "lifecycle-test", metav1.DeleteOptions{})
	require.NoError(t, err)
}
