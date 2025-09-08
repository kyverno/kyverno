package gpol

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestHandleDeleteCreatesUpdateRequest(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test namespace (trigger resource)
	namespace := &unstructured.Unstructured{}
	namespace.SetAPIVersion("v1")
	namespace.SetKind("Namespace")
	namespace.SetName("test-namespace")
	namespace.SetUID("test-namespace-uid")

	// Add namespace to the fake client
	_, err := client.CreateResource(context.TODO(), "v1", "Namespace", "", namespace, false)
	assert.NoError(t, err)

	// Create a test ResourceQuota (generated resource)
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")
	
	// Set the labels that would be set by Kyverno during generation
	resourceQuota.SetLabels(map[string]string{
		common.GeneratePolicyLabel:          "test-policy",
		common.GenerateRuleLabel:            "test-rule",
		common.GenerateSourceUIDLabel:       "test-namespace-uid",
		"app.kubernetes.io/managed-by":      "kyverno",
	})

	// Set up the watcher's metadata cache to simulate the resource being tracked
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}
	wm.dynamicWatchers = map[schema.GroupVersionResource]*watcher{
		gvr: {
			metadataCache: map[types.UID]Resource{
				"test-quota-uid": {
					Name:      "default",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.GeneratePolicyLabel:      "test-policy",
						common.GenerateRuleLabel:        "test-rule",
						common.GenerateSourceUIDLabel:   "test-namespace-uid",
						"app.kubernetes.io/managed-by":  "kyverno",
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete to simulate the ResourceQuota being deleted
	wm.handleDelete(resourceQuota, gvr)

	// Verify that an UpdateRequest was created
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 1, "Expected one UpdateRequest to be created")

	ur := urList.Items[0]
	assert.Equal(t, kyvernov2.Generate, ur.Spec.Type, "UpdateRequest should be of type Generate")
	assert.Equal(t, "test-policy", ur.Spec.Policy, "UpdateRequest should reference the correct policy")
	assert.Equal(t, kyvernov2.Pending, ur.Status.State, "UpdateRequest should be in Pending state")
	
	// Verify that the rule context is set correctly
	assert.Len(t, ur.Spec.RuleContext, 1, "UpdateRequest should have one rule context")
	ruleContext := ur.Spec.RuleContext[0]
	assert.Equal(t, "test-rule", ruleContext.Rule, "Rule context should reference the correct rule")
	assert.Equal(t, "test-namespace", ruleContext.Trigger.Name, "Rule context should reference the trigger namespace")
	assert.Equal(t, "Namespace", ruleContext.Trigger.Kind, "Rule context should reference the correct trigger kind")
}

func TestHandleDeleteFallsBackWhenLabelsAreMissing(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota without proper labels
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")
	
	// Labels are incomplete (missing some required labels)
	resourceQuota.SetLabels(map[string]string{
		"app.kubernetes.io/managed-by": "kyverno",
	})

	// Set up the watcher's metadata cache
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}
	wm.dynamicWatchers = map[schema.GroupVersionResource]*watcher{
		gvr: {
			metadataCache: map[types.UID]Resource{
				"test-quota-uid": {
					Name:      "default",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "kyverno",
						// Missing required labels
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created (fallback behavior)
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when labels are missing")
}
