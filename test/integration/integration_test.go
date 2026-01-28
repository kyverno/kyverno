package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestIntegration(t *testing.T) {
	// 1. Setup a Local Scheme (Best Practice)
	// This ensures we don't rely on global state that might be empty
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kyvernov1.AddToScheme(scheme))

	// 2. Setup the Environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crds")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer testEnv.Stop()

	// 3. Create Client using the Local Scheme
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	t.Run("Create and Verify ClusterPolicy", func(t *testing.T) {
		policyName := "test-policy"
		policy := &kyvernov1.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: policyName,
			},
			Spec: kyvernov1.Spec{
				Rules: []kyvernov1.Rule{
					{
						Name: "match-pods",
						MatchResources: kyvernov1.MatchResources{
							Any: []kyvernov1.ResourceFilter{
								{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod"},
									},
								},
							},
						},
					},
				},
			},
		}

		// Create
		err := k8sClient.Create(context.Background(), policy)
		assert.NoError(t, err)

		// Verify with Debug Logging
		fetchedPolicy := &kyvernov1.ClusterPolicy{}
		assert.Eventually(t, func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: policyName}, fetchedPolicy)
			if err != nil {
				t.Logf("Get failed: %v", err)
				return false
			}
			if fetchedPolicy.Name == "" {
				t.Logf("Get succeeded but Name is empty (decoding issue). Object: %+v", fetchedPolicy)
				return false
			}
			return true
		}, 10*time.Second, 250*time.Millisecond, "Policy should be created and readable")
		
		assert.Equal(t, policyName, fetchedPolicy.Name)
	})
}