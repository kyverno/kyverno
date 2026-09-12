package deleting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	enginecompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	dpolengine "github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
)

func Test_SkipResourceDueToFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := mocks.NewMockConfiguration(ctrl)

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ConfigMap",
	}

	mockConfig.EXPECT().
		ToFilter(gvk, "ConfigMap", "kube-system", "filtered-cm").
		Return(true).
		AnyTimes()

	c := &controller{
		configuration: mockConfig,
	}

	resource := unstructured.Unstructured{}
	resource.SetKind("ConfigMap")
	resource.SetNamespace("kube-system")
	resource.SetName("filtered-cm")

	filtered := c.configuration.ToFilter(
		gvk, resource.GetKind(), resource.GetNamespace(), resource.GetName(),
	)

	assert.True(t, filtered, "Expected resource to be filtered and skipped")
}

// captureQueue wraps a real typed queue but captures the last AddAfter delay used by the controller.
type captureQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	lastDelay      time.Duration
	addAfterCalled bool
}

func (c *captureQueue) AddAfter(item any, delay time.Duration) {
	c.addAfterCalled = true
	c.lastDelay = delay
	c.TypedRateLimitingInterface.AddAfter(item, delay)
}

// Test that deleting controller reconcile clamps the requeue delay when the next execution
// time is in the past (due to an old LastExecutionTime).
func TestReconcile_ClampPastNextExecution(t *testing.T) {
	pol := policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "",
			Name:      "dpol",
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule: "* * * * *",
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"globalcontextentries"},
							},
						},
					},
				},
			},
		},
		Status: policiesv1beta1.DeletingPolicyStatus{
			LastExecutionTime: metav1.NewTime(
				time.Date(1901, 1, 1, 0, 0, 0, 0, time.UTC),
			),
		},
	}
	pol.Name = "dpol"

	fakeClient := versionedfake.NewSimpleClientset(&pol)

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{
			Name: "test-deleting",
		},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	compiler := enginecompiler.NewCompiler()
	provFunc, err := dpolengine.NewProvider(compiler, []policiesv1beta1.DeletingPolicyLike{&pol}, nil)
	if err != nil {
		t.Fatalf("provider init failed: %v", err)
	}
	provider := providerAdapter{fetch: provFunc, name: pol.Name}

	ctrl := &controller{
		kyvernoClient: fakeClient,
		queue:         cq,
		provider:      provider,
	}

	if err := ctrl.reconcile(context.Background(), logr.Discard(), "dpol", "", "dpol"); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	// add a tolerance to the lower bound to account for test flakiness
	if cq.lastDelay < minRequeueDelay-100*time.Millisecond || cq.lastDelay > minRequeueDelay+60*time.Second {
		t.Fatalf("expected delay to next cron minute, got %v", cq.lastDelay)
	}
}

// TestReconcile_DeletingError_CronStillRearmed verifies that queue.AddAfter is called
// even when deleting() returns an error, so the cron schedule survives transient failures.
func TestReconcile_DeletingError_CronStillRearmed(t *testing.T) {
	pol := policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dpol",
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule: "* * * * *",
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{"CREATE"},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
					},
				},
			},
		},
		Status: policiesv1beta1.DeletingPolicyStatus{
			LastExecutionTime: metav1.NewTime(
				time.Date(1901, 1, 1, 0, 0, 0, 0, time.UTC),
			),
		},
	}

	fakeKyvernoClient := versionedfake.NewSimpleClientset(&pol)

	baseQ := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-deleting-err"},
	)
	cq := &captureQueue{TypedRateLimitingInterface: baseQ}

	provider := providerAdapter{
		fetch: func(_ context.Context) ([]dpolengine.Policy, error) {
			return []dpolengine.Policy{{Policy: &pol}}, nil
		},
		name: pol.Name,
	}

	// List returns a non-recoverable error to simulate a transient API server failure.
	// Register the configmaps list kind so the fake dynamic client reaches the reactor.
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMapList"}, &unstructured.UnstructuredList{})
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)
	dynClient.PrependReactor("list", "*", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("connection refused")
	})
	dClient := dclient.NewFakeClientWithDisco(dynClient, kubefake.NewSimpleClientset(), dclient.NewFakeDiscoveryClient(nil))

	c := &controller{
		client:        dClient,
		kyvernoClient: fakeKyvernoClient,
		queue:         cq,
		provider:      provider,
	}

	err := c.reconcile(context.Background(), logr.Discard(), "dpol", "", "dpol")

	assert.Error(t, err, "reconcile must surface the deleting error so the workqueue can rate-limit retries")
	assert.True(t, cq.addAfterCalled, "AddAfter must be called even when deleting fails so the cron schedule survives")
}

type providerAdapter struct {
	fetch dpolengine.ProviderFunc
	name  string
}

func (p providerAdapter) Get(ctx context.Context, namespace, name string) (dpolengine.Policy, error) {
	list, err := p.fetch.Fetch(ctx)
	if err != nil {
		return dpolengine.Policy{}, err
	}
	for _, it := range list {
		if it.Policy.GetName() == name {
			return it, nil
		}
	}
	return dpolengine.Policy{}, fmt.Errorf("not found")
}
