package policy

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestUnlabelDownstream(t *testing.T) {
	// Create a mock unstructured object with Kyverno tracking labels
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetNamespace("default")
	u.SetName("target-cm")
	u.SetUID(types.UID("downstream-uid"))
	u.SetLabels(map[string]string{
		common.GeneratePolicyLabel:          "test-policy",
		common.GeneratePolicyNamespaceLabel: "default",
		common.GenerateRuleLabel:            "test-rule",
		"other-label":                       "keep-me",
	})

	// Setup fake client with the mock object
	client, err := dclient.NewFakeClient(runtime.NewScheme(), nil, u)
	require.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	// Initialize the policy controller
	controller := &policyController{
		client: client,
		log:    logging.WithName("policy-test"),
	}

	// Create the selector to trigger unlabelDownstream
	selector := updatedResource{
		policy:          "test-policy",
		policyNamespace: "default",
		ruleResources: []ruleResource{
			{
				rule:  "test-rule",
				kinds: []string{"ConfigMap"},
			},
		},
	}

	// Call the function
	controller.unlabelDownstream(selector)

	// Fetch the updated resource and verify labels
	target, err := client.GetResource(context.TODO(), "v1", "ConfigMap", "default", "target-cm")
	require.NoError(t, err)

	labels := target.GetLabels()
	assert.NotNil(t, labels)
	assert.Equal(t, "keep-me", labels["other-label"], "Other labels should not be removed")
	assert.NotContains(t, labels, common.GeneratePolicyLabel, "GeneratePolicyLabel should be removed")
	assert.NotContains(t, labels, common.GeneratePolicyNamespaceLabel, "GeneratePolicyNamespaceLabel should be removed")
	assert.NotContains(t, labels, common.GenerateRuleLabel, "GenerateRuleLabel should be removed")
}
