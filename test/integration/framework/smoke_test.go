package framework_test

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRequireLabelsPolicy(name string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{{
				Name: "check-labels",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"Pod"},
					},
				},
				Validation: &kyvernov1.Validation{
					Message: "label 'app' is required",
					RawPattern: &apiextv1.JSON{
						Raw: []byte(`{"metadata":{"labels":{"app":"?*"}}}`),
					},
				},
			}},
		},
	}
}

func TestEnvTestBootstrap(t *testing.T) {
	cfg, _ := framework.SetupEnvTest(t)
	assert.NotNil(t, cfg, "envtest config should not be nil")
	assert.NotEmpty(t, cfg.Host, "API server host should be set")
}

func TestClusterPolicy_CRUD(t *testing.T) {
	cfg, _ := framework.SetupEnvTest(t)
	_, kyvernoClient := framework.NewClients(t, cfg)

	policy := newRequireLabelsPolicy("test-require-labels")

	// Create
	created, err := kyvernoClient.KyvernoV1().ClusterPolicies().Create(
		context.Background(), policy, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "test-require-labels", created.Name)
	assert.Len(t, created.Spec.Rules, 1)

	// Read
	fetched, err := kyvernoClient.KyvernoV1().ClusterPolicies().Get(
		context.Background(), created.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "check-labels", fetched.Spec.Rules[0].Name)

	// Update
	fetched.Spec.Rules[0].Validation.Message = "updated message"
	updated, err := kyvernoClient.KyvernoV1().ClusterPolicies().Update(
		context.Background(), fetched, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Equal(t, "updated message", updated.Spec.Rules[0].Validation.Message)

	// Delete
	err = kyvernoClient.KyvernoV1().ClusterPolicies().Delete(
		context.Background(), created.Name, metav1.DeleteOptions{})
	require.NoError(t, err)

	// Verify deletion
	_, err = kyvernoClient.KyvernoV1().ClusterPolicies().Get(
		context.Background(), created.Name, metav1.GetOptions{})
	assert.Error(t, err, "policy should not exist after deletion")
}

func TestNamespaceIsolation(t *testing.T) {
	cfg, _ := framework.SetupEnvTest(t)
	kubeClient, _ := framework.NewClients(t, cfg)

	ns := framework.UniqueNamespace(t, kubeClient)
	assert.NotEmpty(t, ns.Name)

	// Verify namespace exists in the API server
	fetched, err := kubeClient.CoreV1().Namespaces().Get(
		context.Background(), ns.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, ns.Name, fetched.Name)
}

func TestClusterPolicy_List(t *testing.T) {
	cfg, _ := framework.SetupEnvTest(t)
	_, kyvernoClient := framework.NewClients(t, cfg)

	// Create two policies
	framework.CreateClusterPolicy(t, kyvernoClient, newRequireLabelsPolicy("policy-a"))
	framework.CreateClusterPolicy(t, kyvernoClient, newRequireLabelsPolicy("policy-b"))

	// List should return both
	list, err := kyvernoClient.KyvernoV1().ClusterPolicies().List(
		context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, list.Items, 2)
}
