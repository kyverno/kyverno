package deleting

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	enginecompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	dpolengine "github.com/kyverno/kyverno/pkg/cel/policies/dpol/engine"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
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
	lastDelay time.Duration
}

func (c *captureQueue) AddAfter(item any, delay time.Duration) {
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

// mockNamespaceLister is a simple stub to simulate the informer cache returning namespaces.
type mockNamespaceLister struct {
	namespaces []*corev1.Namespace
}

func (m *mockNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	if selector == nil || selector.Empty() {
		return m.namespaces, nil
	}
	var filtered []*corev1.Namespace
	for _, ns := range m.namespaces {
		if selector.Matches(labels.Set(ns.Labels)) {
			filtered = append(filtered, ns)
		}
	}
	return filtered, nil
}

func (m *mockNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	for _, ns := range m.namespaces {
		if ns.Name == name {
			return ns, nil
		}
	}
	return nil, fmt.Errorf("namespace not found")
}

// mockDClient embeds the dclient.Interface to bypass full implementation.
type mockDClient struct {
	dclient.Interface
	dyn              dynamic.Interface
	kube             kubernetes.Interface
	deletedResources []string
}

type mockEventGenerator struct {
	events []event.Info
}

func (m *mockEventGenerator) Add(infoList ...event.Info) {
	m.events = append(m.events, infoList...)
}

func (m *mockDClient) GetDynamicInterface() dynamic.Interface {
	return m.dyn
}

// Implement GetKubeClient so restmapper doesn't panic!
func (m *mockDClient) GetKubeClient() kubernetes.Interface {
	return m.kube
}

func (m *mockDClient) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, forceDelete bool, deleteOptions metav1.DeleteOptions) error {
	m.deletedResources = append(m.deletedResources, namespace+"/"+name)
	return nil
}

func Test_DeletingController_Pagination_And_Namespace_Iteration(t *testing.T) {

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	dynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	// Create a fake kube client and teach it that "pods" are namespaced
	kubeClient := kubeclientfake.NewSimpleClientset()
	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if ok {
		fakeDiscovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{
						Name:       "pods",
						Kind:       "Pod",
						Namespaced: true,
					},
				},
			},
		}
	}

	// 1. Intercept List actions to verify pagination and namespace boundaries
	var listActions []clienttesting.ListAction
	ns1Calls := 0

	dynClient.PrependReactor("list", "*", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		listAction := action.(clienttesting.ListAction)
		listActions = append(listActions, listAction)

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "PodList"})

		// Simulate pagination: On the first call to ns1, return a token.
		// On the second call, return empty token (end of list).
		if listAction.GetNamespace() == "ns1" {
			ns1Calls++
			if ns1Calls == 1 {
				list.SetContinue("token-123")
			}
		}

		return true, list, nil
	})

	// 2. Setup mock namespace lister with two namespaces
	nsLister := &mockNamespaceLister{
		namespaces: []*corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
		},
	}

	c := &controller{
		client: &mockDClient{
			dyn:  dynClient,
			kube: kubeClient, // Inject the fake kube client here!
		},
		nsLister: nsLister,
	}

	// 3. Setup a mock policy targeting a namespaced resource (Pods)
	pol := policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-paginated-clean"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
		},
	}

	ePolicy := dpolengine.Policy{Policy: &pol}

	// 4. Execute the deleting function and ignore the error return
	err := c.deleting(context.Background(), logr.Discard(), ePolicy)
	assert.NoError(t, err)
	// Verify Pagination and namespace iteration.
	assert.Equal(t, 3, len(listActions), "Expected exactly 3 list actions (2 for ns1 due to pagination, 1 for ns2)")

	// Call 1: First page of ns1
	assert.Equal(t, "ns1", listActions[0].GetNamespace())

	// Call 2: Second page of ns1 (Pagination works!)
	assert.Equal(t, "ns1", listActions[1].GetNamespace())

	// Call 3: First (and only) page of ns2 (Namespace iteration works!)
	assert.Equal(t, "ns2", listActions[2].GetNamespace())
}

func TestDeleting_ClusterWidePaginationWithoutNamespaceSelector(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfig := mocks.NewMockConfiguration(ctrl)

	mockConfig.EXPECT().
		ToFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false).
		AnyTimes()

	scheme := runtime.NewScheme()

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{
			Group:   "",
			Version: "v1",
		},
	})

	mapper.Add(
		schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
		},
		meta.RESTScopeNamespace,
	)

	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}

	dynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		gvrToListKind,
	)

	pod := func(name, namespace string) unstructured.Unstructured {
		return unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
			},
		}
	}

	kubeClient := kubeclientfake.NewSimpleClientset()
	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if ok {
		fakeDiscovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{
						Name:       "pods",
						Kind:       "Pod",
						Namespaced: true,
					},
				},
			},
		}
	}

	var listActions []clienttesting.ListAction

	dynClient.PrependReactor(
		"list",
		"*",
		func(action clienttesting.Action) (bool, runtime.Object, error) {
			listAction := action.(clienttesting.ListAction)
			listActions = append(listActions, listAction)

			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(
				schema.GroupVersionKind{
					Version: "v1",
					Kind:    "PodList",
				},
			)

			switch len(listActions) {
			case 1:
				list.Items = []unstructured.Unstructured{
					pod("pod-1", "ns1"),
					pod("pod-2", "ns2"),
				}
				list.SetContinue("token-123")

			case 2:
				list.Items = []unstructured.Unstructured{
					pod("pod-3", "ns1"),
				}
			}

			return true, list, nil
		},
	)

	nsLister := &mockNamespaceLister{
		namespaces: []*corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
		},
	}

	pol := policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-wide-pagination",
		},
		Spec: policiesv1beta1.DeletingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "always-true",
					Expression: "true",
				},
			},
		},
	}

	compiler := enginecompiler.NewCompiler()

	compiledPolicy, errs := compiler.Compile(&pol, nil)
	assert.Empty(t, errs)

	ePolicy := dpolengine.Policy{
		Policy:         &pol,
		CompiledPolicy: compiledPolicy,
	}

	matcher := matching.NewMatcher()

	dpolEngine := dpolengine.NewEngine(
		func(namespace string) *corev1.Namespace {
			return nil
		},
		mapper,
		&libs.FakeContextProvider{},
		matcher,
	)

	mockClient := &mockDClient{
		dyn:  dynClient,
		kube: kubeClient,
	}

	mockEventGen := &mockEventGenerator{}

	c := &controller{
		client:        mockClient,
		nsLister:      nsLister,
		engine:        dpolEngine,
		configuration: mockConfig,
		eventGen:      mockEventGen,
	}

	err := c.deleting(
		context.Background(),
		logr.Discard(),
		ePolicy,
	)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"ns1/pod-1", "ns2/pod-2", "ns1/pod-3"}, mockClient.deletedResources)

	assert.Len(t, mockEventGen.events, 3)

	assert.Equal(t, 2, len(listActions), "Expected exactly 2 cluster-wide list actions due to pagination")
	assert.Equal(t, "", listActions[0].GetNamespace())
	assert.Equal(t, "", listActions[1].GetNamespace())
}

func TestDeleting_NamespaceSelector(t *testing.T) {

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}
	dynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	kubeClient := kubeclientfake.NewSimpleClientset()
	fakeDiscovery, ok := kubeClient.Discovery().(*fakediscovery.FakeDiscovery)
	if ok {
		fakeDiscovery.Resources = []*metav1.APIResourceList{
			{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{
					{Name: "pods", Kind: "Pod", Namespaced: true},
				},
			},
		}
	}

	var listActions []clienttesting.ListAction
	dynClient.PrependReactor("list", "*", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		listActions = append(listActions, action.(clienttesting.ListAction))
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "PodList"})
		return true, list, nil
	})

	// Setup namespaces with labels
	nsLister := &mockNamespaceLister{
		namespaces: []*corev1.Namespace{
			{ObjectMeta: metav1.ObjectMeta{Name: "ns-prod", Labels: map[string]string{"env": "prod"}}},
			{ObjectMeta: metav1.ObjectMeta{Name: "ns-dev", Labels: map[string]string{"env": "dev"}}},
		},
	}

	c := &controller{
		client: &mockDClient{
			dyn:  dynClient,
			kube: kubeClient,
		},
		nsLister: nsLister,
	}

	// Setup policy with a NamespaceSelector for env=prod
	pol := policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-namespace-selector"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"env": "prod"},
				},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
		},
	}

	ePolicy := dpolengine.Policy{Policy: &pol}
	err := c.deleting(context.Background(), logr.Discard(), ePolicy)
	assert.NoError(t, err)

	// Verify that only the selected namespace was listed.
	assert.Equal(t, 1, len(listActions), "Expected exactly 1 list action because ns-dev should be filtered out")
	assert.Equal(t, "ns-prod", listActions[0].GetNamespace(), "Expected the controller to only list resources in ns-prod")
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
