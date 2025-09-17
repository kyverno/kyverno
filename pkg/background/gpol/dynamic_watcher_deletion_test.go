package gpol

import (
	"context"
	"errors"
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
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
		common.GeneratePolicyLabel:    "test-policy",
		common.GenerateRuleLabel:      "test-rule",
		common.GenerateSourceUIDLabel: "test-namespace-uid",
		kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
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
						common.GeneratePolicyLabel:    "test-policy",
						common.GenerateRuleLabel:      "test-rule",
						common.GenerateSourceUIDLabel: "test-namespace-uid",
						kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
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
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
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
						kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
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

func TestHandleDeleteWithNilLabelsInCache(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota with managed-by label (so it takes the managed path)
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")
	resourceQuota.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
	})

	// Set up the watcher's metadata cache with nil labels to test that path
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}
	wm.dynamicWatchers = map[schema.GroupVersionResource]*watcher{
		gvr: {
			metadataCache: map[types.UID]Resource{
				"test-quota-uid": {
					Name:      "default",
					Namespace: "test-namespace",
					Labels:    nil, // nil labels in cache to test this path
					Data:      resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete (should skip due to nil labels in cache)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created (early return due to nil labels)
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when labels in cache are nil")
}

func TestHandleDeleteErrorGettingSourceResource(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota with complete labels
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")

	// Set complete labels
	resourceQuota.SetLabels(map[string]string{
		common.GeneratePolicyLabel:     "test-policy",
		common.GenerateRuleLabel:       "test-rule",
		common.GenerateSourceUIDLabel:  "test-namespace-uid",
		"app.kubernetes.io/managed-by": "kyverno",
	})

	// DON'T add the namespace to the client to simulate error

	// Set up the watcher's metadata cache
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}
	wm.dynamicWatchers = map[schema.GroupVersionResource]*watcher{
		gvr: {
			metadataCache: map[types.UID]Resource{
				"test-quota-uid": {
					Name:      "default",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.GeneratePolicyLabel:    "test-policy",
						common.GenerateRuleLabel:      "test-rule",
						common.GenerateSourceUIDLabel: "test-namespace-uid",
						kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete (should fail to get source resource)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created due to error getting source resource
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when source resource cannot be found")
}

func TestHandleDeleteResourceNotInCache(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")
	resourceQuota.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
	})

	// Set up the watcher but DON'T add the resource to metadata cache
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}
	wm.dynamicWatchers = map[schema.GroupVersionResource]*watcher{
		gvr: {
			metadataCache: map[types.UID]Resource{
				// Resource not in cache
			},
		},
	}

	// Test: Call handleDelete (should be no-op since resource not in cache)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when resource not in cache")
}

func TestHandleDeleteNoWatcherExists(t *testing.T) {
	// Setup
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")
	resourceQuota.SetLabels(map[string]string{
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
	})

	// DON'T set up any watchers
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}

	// Test: Call handleDelete (should be no-op since no watcher exists)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when no watcher exists")
}

func TestHandleDeleteErrorCreatingUpdateRequest(t *testing.T) {
	// Setup with fake client that will error on UpdateRequest creation
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()

	// Add reaction to simulate error on UpdateRequest creation
	kyvernoClient.Fake.PrependReactor("create", "updaterequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("simulated create error")
	})

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
		common.GeneratePolicyLabel:    "test-policy",
		common.GenerateRuleLabel:      "test-rule",
		common.GenerateSourceUIDLabel: "test-namespace-uid",
		kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
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
						common.GeneratePolicyLabel:    "test-policy",
						common.GenerateRuleLabel:      "test-rule",
						common.GenerateSourceUIDLabel: "test-namespace-uid",
						kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete (should handle the error gracefully)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that no UpdateRequest was created due to the error
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when creation fails")
}

func TestHandleDeleteErrorUpdatingStatus(t *testing.T) {
	// Setup with fake client that will error on UpdateRequest status update
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()

	// Add reaction to simulate error on UpdateRequest status update
	kyvernoClient.Fake.PrependReactor("update", "updaterequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "status" {
			return true, nil, errors.New("simulated status update error")
		}
		return false, nil, nil
	})

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
		common.GeneratePolicyLabel:    "test-policy",
		common.GenerateRuleLabel:      "test-rule",
		common.GenerateSourceUIDLabel: "test-namespace-uid",
		kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
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
						common.GeneratePolicyLabel:    "test-policy",
						common.GenerateRuleLabel:      "test-rule",
						common.GenerateSourceUIDLabel: "test-namespace-uid",
						kyverno.LabelAppManagedBy:     kyverno.ValueKyvernoApp,
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete (should create UR but fail to update status)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that UpdateRequest was created but status update failed
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 1, "UpdateRequest should be created even if status update fails")

	ur := urList.Items[0]
	assert.Equal(t, kyvernov2.Generate, ur.Spec.Type)
	assert.Equal(t, "test-policy", ur.Spec.Policy)
	// Status should be empty/default since the update failed
	assert.Equal(t, kyvernov2.UpdateRequestState(""), ur.Status.State, "Status should be empty due to failed update")
}

func TestHandleDeleteFallbackWithCreationError(t *testing.T) {
	// Setup with fake client that will error on resource creation
	client := dclient.NewEmptyFakeClient()
	kyvernoClient := kyvernoclient.NewSimpleClientset()
	log := logging.WithName("test-logging")
	wm := NewWatchManager(log, client, kyvernoClient)

	// Create a test ResourceQuota with incomplete labels (triggering fallback)
	resourceQuota := &unstructured.Unstructured{}
	resourceQuota.SetAPIVersion("v1")
	resourceQuota.SetKind("ResourceQuota")
	resourceQuota.SetName("default")
	resourceQuota.SetNamespace("test-namespace")
	resourceQuota.SetUID("test-quota-uid")

	// Missing some required labels to trigger fallback
	resourceQuota.SetLabels(map[string]string{
		common.GeneratePolicyLabel: "test-policy",
		// Missing other required labels
		kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
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
						common.GeneratePolicyLabel: "test-policy",
						// Missing other required labels
						kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
					},
					Data: resourceQuota,
				},
			},
		},
	}

	// Test: Call handleDelete (should trigger fallback and handle creation error)
	wm.handleDelete(resourceQuota, gvr)

	// Verify that NO UpdateRequest was created (fallback path was taken)
	urList, err := kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).List(context.TODO(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, urList.Items, 0, "No UpdateRequest should be created when fallback is used")
}
