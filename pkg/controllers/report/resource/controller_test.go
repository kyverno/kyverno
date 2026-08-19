package resource

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

type stubListClient struct {
	calls   []metav1.ListOptions
	handler func(call int, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

func (s *stubListClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	n := len(s.calls)
	s.calls = append(s.calls, opts)
	return s.handler(n, opts)
}

func configMapObject(name, uid string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
			"uid":       uid,
		},
		"data": map[string]any{"k": name},
	}}
}

func TestListResourcesPaginated_MultiplePages(t *testing.T) {
	first := configMapObject("cm-1", "uid-1")
	second := configMapObject("cm-2", "uid-2")
	client := &stubListClient{handler: func(call int, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		assert.Equal(t, int64(listPageSize), opts.Limit)
		list := &unstructured.UnstructuredList{}
		list.SetResourceVersion("10245")
		switch call {
		case 0:
			assert.Empty(t, opts.Continue)
			list.Items = []unstructured.Unstructured{first}
			list.SetContinue("token-1")
			return list, nil
		case 1:
			assert.Equal(t, "token-1", opts.Continue)
			list.Items = []unstructured.Unstructured{second}
			return list, nil
		default:
			t.Fatalf("unexpected list call %d", call)
			return nil, nil
		}
	}}

	hashes := map[types.UID]Resource{}
	rv, err := listResourcesPaginated(context.Background(), client, hashes)
	require.NoError(t, err)
	assert.Equal(t, "10245", rv)
	assert.Len(t, client.calls, 2)
	assert.Len(t, hashes, 2)
	assert.Equal(t, "cm-1", hashes["uid-1"].Name)
	assert.Equal(t, "cm-2", hashes["uid-2"].Name)
	assert.NotEmpty(t, hashes["uid-1"].Hash)
	assert.NotEmpty(t, hashes["uid-2"].Hash)
	assert.NotEqual(t, hashes["uid-1"].Hash, hashes["uid-2"].Hash)
}

func TestListResourcesPaginated_ExpiredContinueRestarts(t *testing.T) {
	first := configMapObject("cm-1", "uid-1")
	second := configMapObject("cm-2", "uid-2")
	stale := configMapObject("stale", "uid-stale")
	client := &stubListClient{handler: func(call int, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		list := &unstructured.UnstructuredList{}
		list.SetResourceVersion("200")
		switch call {
		case 0:
			list.Items = []unstructured.Unstructured{stale}
			list.SetContinue("expired-token")
			return list, nil
		case 1:
			assert.Equal(t, "expired-token", opts.Continue)
			return nil, apierrors.NewResourceExpired("continue token expired")
		case 2:
			assert.Empty(t, opts.Continue)
			list.Items = []unstructured.Unstructured{first}
			list.SetContinue("token-2")
			return list, nil
		case 3:
			assert.Equal(t, "token-2", opts.Continue)
			list.Items = []unstructured.Unstructured{second}
			return list, nil
		default:
			t.Fatalf("unexpected list call %d", call)
			return nil, nil
		}
	}}

	hashes := map[types.UID]Resource{}
	rv, err := listResourcesPaginated(context.Background(), client, hashes)
	require.NoError(t, err)
	assert.Equal(t, "200", rv)
	assert.Len(t, client.calls, 4)
	assert.Len(t, hashes, 2)
	_, hasStale := hashes["uid-stale"]
	assert.False(t, hasStale)
	assert.Contains(t, hashes, types.UID("uid-1"))
	assert.Contains(t, hashes, types.UID("uid-2"))
}

type listWatchDiscovery struct {
	dclient.IDiscovery
}

func (d listWatchDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	result, err := d.IDiscovery.FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	for key, api := range result {
		api.Verbs = []string{"list", "watch", "get"}
		result[key] = api
	}
	return result, nil
}

func configMapMatchConstraints() *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"configmaps"},
				},
			},
		}},
	}
}

func newValidatingPolicy(name string, background *bool) *policiesv1beta1.ValidatingPolicy {
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: configMapMatchConstraints(),
		},
	}
	if background != nil {
		pol.Spec.EvaluationConfiguration = &policiesv1beta1.EvaluationConfiguration{
			Background: &policiesv1beta1.BackgroundConfiguration{
				Enabled: background,
			},
		}
	}
	return pol
}

func newClusterPolicy(name string, background *bool) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kyvernov1.SchemeGroupVersion.String(),
			Kind:       "ClusterPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: kyvernov1.Spec{
			Background: background,
			Rules: []kyvernov1.Rule{{
				Name: "check",
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{"ConfigMap"},
					},
				},
				Validation: &kyvernov1.Validation{Message: "require a value"},
			}},
		},
	}
}

func countConfigMapLists(t *testing.T, dyn dynamic.Interface) int {
	t.Helper()
	fakeDyn, ok := dyn.(*dynamicfake.FakeDynamicClient)
	require.True(t, ok, "expected FakeDynamicClient")
	count := 0
	for _, action := range fakeDyn.Actions() {
		list, ok := action.(k8stesting.ListAction)
		if !ok {
			continue
		}
		if list.GetResource().Resource == "configmaps" {
			count++
		}
	}
	return count
}

func setupController(t *testing.T, objects ...runtime.Object) (*controller, dynamic.Interface) {
	t.Helper()
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-1",
			Namespace: "default",
			UID:       "uid-1",
		},
	}
	scheme := runtime.NewScheme()
	dClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{
		{Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}, cm)
	require.NoError(t, err)
	dClient.SetDiscovery(listWatchDiscovery{IDiscovery: dclient.NewFakeDiscoveryClient(nil)})

	polIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	cpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, obj := range objects {
		switch policy := obj.(type) {
		case *kyvernov1.Policy:
			require.NoError(t, polIndexer.Add(policy))
		case *kyvernov1.ClusterPolicy:
			require.NoError(t, cpolIndexer.Add(policy))
		case *policiesv1beta1.ValidatingPolicy:
			require.NoError(t, vpolIndexer.Add(policy))
		default:
			t.Fatalf("unsupported policy type %T", obj)
		}
	}

	ctrl := &controller{
		client:          dClient,
		polLister:       kyvernov1listers.NewPolicyLister(polIndexer),
		cpolLister:      kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		vpolLister:      policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
		queue:           workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]()),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
	}
	t.Cleanup(func() {
		ctrl.stopDynamicWatchers()
		ctrl.queue.ShutDown()
	})
	return ctrl, dClient.GetDynamicInterface()
}

func TestWarmupSkipsBackgroundDisabledCELPolicy(t *testing.T) {
	ctrl, dyn := setupController(t, newValidatingPolicy("admission-only", ptr.To(false)))
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Zero(t, countConfigMapLists(t, dyn))
}

func TestWarmupWatchesBackgroundEnabledCELPolicy(t *testing.T) {
	ctrl, dyn := setupController(t, newValidatingPolicy("background-on", ptr.To(true)))
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Greater(t, countConfigMapLists(t, dyn), 0)
}

func TestWarmupWatchesDefaultBackgroundCELPolicy(t *testing.T) {
	ctrl, dyn := setupController(t, newValidatingPolicy("background-default", nil))
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Greater(t, countConfigMapLists(t, dyn), 0)
}

func TestWarmupSkipsBackgroundDisabledLegacyPolicy(t *testing.T) {
	ctrl, dyn := setupController(t, newClusterPolicy("legacy-admission-only", ptr.To(false)))
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Zero(t, countConfigMapLists(t, dyn))
}

func TestWarmupWatchesBackgroundEnabledLegacyPolicy(t *testing.T) {
	ctrl, dyn := setupController(t, newClusterPolicy("legacy-background-on", ptr.To(true)))
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Greater(t, countConfigMapLists(t, dyn), 0)
}

func TestWarmupKeepsSharedGVRWhenAnyBackgroundPolicyMatches(t *testing.T) {
	ctrl, dyn := setupController(t,
		newValidatingPolicy("admission-only", ptr.To(false)),
		newValidatingPolicy("background-on", ptr.To(true)),
	)
	require.NoError(t, ctrl.Warmup(context.Background()))
	assert.Greater(t, countConfigMapLists(t, dyn), 0)
}
