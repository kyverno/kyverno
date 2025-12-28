package cleanup

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/utils/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_CleanupPolicy_WithEnvTest(t *testing.T) {
	// 1. Setup EnvTest
	env, err := testutil.SetupEnvTest()
	require.NoError(t, err)
	defer env.Stop()

	ctx := context.Background()

	// 2. Define a CleanupPolicy
	policy := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "envtest-policy",
			Namespace: "default",
		},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "*/5 * * * *",
		},
	}

	// 3. Create using the real (envtest) client
	err = env.Client.Create(ctx, policy)
	require.NoError(t, err)

	// 4. Verify creation
	retrieved := &kyvernov2.CleanupPolicy{}
	err = env.Client.Get(ctx, client.ObjectKey{Name: "envtest-policy", Namespace: "default"}, retrieved)
	require.NoError(t, err)
	assert.Equal(t, "envtest-policy", retrieved.Name)
	assert.Equal(t, "*/5 * * * *", retrieved.Spec.Schedule)

	// 5. Update
	retrieved.Spec.Schedule = "*/10 * * * *"
	err = env.Client.Update(ctx, retrieved)
	require.NoError(t, err)

	// 6. Verify Update
	updated := &kyvernov2.CleanupPolicy{}
	err = env.Client.Get(ctx, client.ObjectKey{Name: "envtest-policy", Namespace: "default"}, updated)
	require.NoError(t, err)
	assert.Equal(t, "*/10 * * * *", updated.Spec.Schedule)
}
