//go:build envtest
// +build envtest

package cleanup

import (
	"context"
	"fmt"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/utils/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test_CleanupPolicy_IntegrationWithRealAPIServer tests CleanupPolicy CRUD operations
// against a real Kubernetes API server (envtest). This validates:
// - CRD schema validation
// - API server interactions
// - Resource lifecycle management
func Test_CleanupPolicy_IntegrationWithRealAPIServer(t *testing.T) {
	// Setup envtest with real API server and CRDs
	env, err := testutil.SetupEnvTest()
	require.NoError(t, err)
	defer env.Stop()

	ctx := context.Background()

	t.Run("Create policy with valid schema", func(t *testing.T) {
		policy := &kyvernov2.CleanupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-valid-policy",
				Namespace: "default",
			},
			Spec: kyvernov2.CleanupPolicySpec{
				Schedule: "*/5 * * * *",
				MatchResources: kyvernov2.MatchResources{
					Any: kyvernov1.ResourceFilters{
						{
							ResourceDescription: kyvernov1.ResourceDescription{
								Kinds: []string{"Pod"},
							},
						},
					},
				},
			},
		}

		err := env.Client.Create(ctx, policy)
		require.NoError(t, err, "Should create policy with valid schema")

		// Verify it was created
		var created kyvernov2.CleanupPolicy
		err = env.Client.Get(ctx, client.ObjectKeyFromObject(policy), &created)
		require.NoError(t, err)
		assert.Equal(t, "*/5 * * * *", created.Spec.Schedule)
	})

	t.Run("Update policy schedule", func(t *testing.T) {
		// Get the policy
		policy := &kyvernov2.CleanupPolicy{}
		err := env.Client.Get(ctx, client.ObjectKey{
			Name:      "test-valid-policy",
			Namespace: "default",
		}, policy)
		require.NoError(t, err)

		// Update schedule
		policy.Spec.Schedule = "*/10 * * * *"
		err = env.Client.Update(ctx, policy)
		require.NoError(t, err)

		// Verify update
		var updated kyvernov2.CleanupPolicy
		err = env.Client.Get(ctx, client.ObjectKeyFromObject(policy), &updated)
		require.NoError(t, err)
		assert.Equal(t, "*/10 * * * *", updated.Spec.Schedule)
	})

	t.Run("List policies in namespace", func(t *testing.T) {
		policyList := &kyvernov2.CleanupPolicyList{}
		err := env.Client.List(ctx, policyList, client.InNamespace("default"))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(policyList.Items), 1, "Should have at least one policy")
	})

	t.Run("Delete policy", func(t *testing.T) {
		policy := &kyvernov2.CleanupPolicy{}
		err := env.Client.Get(ctx, client.ObjectKey{
			Name:      "test-valid-policy",
			Namespace: "default",
		}, policy)
		require.NoError(t, err)

		err = env.Client.Delete(ctx, policy)
		require.NoError(t, err)

		// Verify deletion
		require.Eventually(t, func() bool {
			var deleted kyvernov2.CleanupPolicy
			err := env.Client.Get(ctx, client.ObjectKeyFromObject(policy), &deleted)
			return errors.IsNotFound(err)
		}, 10*time.Second, 500*time.Millisecond, "Policy should be deleted")
	})
}

// Test_ClusterCleanupPolicy_IntegrationWithRealAPIServer tests ClusterCleanupPolicy operations
func Test_ClusterCleanupPolicy_IntegrationWithRealAPIServer(t *testing.T) {
	env, err := testutil.SetupEnvTest()
	require.NoError(t, err)
	defer env.Stop()

	ctx := context.Background()

	t.Run("Create cluster-scoped cleanup policy", func(t *testing.T) {
		policy := &kyvernov2.ClusterCleanupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster-policy",
			},
			Spec: kyvernov2.CleanupPolicySpec{
				Schedule: "0 * * * *",
				MatchResources: kyvernov2.MatchResources{
					Any: kyvernov1.ResourceFilters{
						{
							ResourceDescription: kyvernov1.ResourceDescription{
								Kinds: []string{"ConfigMap"},
							},
						},
					},
				},
			},
		}

		err := env.Client.Create(ctx, policy)
		require.NoError(t, err)

		// Verify
		var created kyvernov2.ClusterCleanupPolicy
		err = env.Client.Get(ctx, client.ObjectKey{Name: policy.Name}, &created)
		require.NoError(t, err)
		assert.Equal(t, "0 * * * *", created.Spec.Schedule)

		// Cleanup
		err = env.Client.Delete(ctx, policy)
		require.NoError(t, err)
	})
}

// Test_ConcurrentPolicyOperations tests multiple policies being created/updated concurrently
func Test_ConcurrentPolicyOperations(t *testing.T) {
	env, err := testutil.SetupEnvTest()
	require.NoError(t, err)
	defer env.Stop()

	ctx := context.Background()

	numPolicies := 5
	errChan := make(chan error, numPolicies)

	t.Run("Create multiple policies concurrently", func(t *testing.T) {
		for i := 0; i < numPolicies; i++ {
			go func(index int) {
				policy := &kyvernov2.CleanupPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("concurrent-test-%d", index),
						Namespace: "default",
					},
					Spec: kyvernov2.CleanupPolicySpec{
						Schedule: "*/5 * * * *",
					},
				}
				errChan <- env.Client.Create(ctx, policy)
			}(i)
		}

		// Wait for all creates
		for i := 0; i < numPolicies; i++ {
			err := <-errChan
			assert.NoError(t, err, "Concurrent creation should succeed")
		}

		// Verify all were created
		policyList := &kyvernov2.CleanupPolicyList{}
		err := env.Client.List(ctx, policyList, client.InNamespace("default"))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(policyList.Items), numPolicies)
	})

	t.Run("Update policies concurrently", func(t *testing.T) {
		for i := 0; i < numPolicies; i++ {
			go func(index int) {
				policy := &kyvernov2.CleanupPolicy{}
				key := client.ObjectKey{
					Name:      fmt.Sprintf("concurrent-test-%d", index),
					Namespace: "default",
				}
				
				if err := env.Client.Get(ctx, key, policy); err != nil {
					errChan <- err
					return
				}

				policy.Spec.Schedule = "*/15 * * * *"
				errChan <- env.Client.Update(ctx, policy)
			}(i)
		}

		// Wait for all updates
		for i := 0; i < numPolicies; i++ {
			err := <-errChan
			assert.NoError(t, err, "Concurrent updates should succeed")
		}
	})
}

// Test_PolicySchemaValidation tests that API server validates policy schemas correctly
func Test_PolicySchemaValidation(t *testing.T) {
	env, err := testutil.SetupEnvTest()
	require.NoError(t, err)
	defer env.Stop()

	ctx := context.Background()

	t.Run("Valid cron schedule accepted", func(t *testing.T) {
		policy := &kyvernov2.CleanupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-schedule",
				Namespace: "default",
			},
			Spec: kyvernov2.CleanupPolicySpec{
				Schedule: "0 0 * * *", // Valid cron
			},
		}

		err := env.Client.Create(ctx, policy)
		assert.NoError(t, err, "Valid schedule should be accepted")

		// Cleanup
		env.Client.Delete(ctx, policy)
	})
}
