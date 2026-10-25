package policycache

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	pcache "github.com/kyverno/kyverno/pkg/policycache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type fakeCache struct {
	mu         sync.Mutex
	setCalls   []string
	unsetCalls []string
	setErr     error
}

func (f *fakeCache) Set(key string, _ kyvernov1.PolicyInterface, _ pcache.ResourceFinder) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCalls = append(f.setCalls, key)
	return f.setErr
}

func (f *fakeCache) Unset(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsetCalls = append(f.unsetCalls, key)
}

func (f *fakeCache) GetPolicies(_ pcache.PolicyType, _ schema.GroupVersionResource, _ string, _ *corev1.Namespace) []kyvernov1.PolicyInterface {
	return nil
}

func (f *fakeCache) getSetCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.setCalls...)
}

func (f *fakeCache) getUnsetCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.unsetCalls...)
}

type erroringCpolLister struct {
	kyvernov1listers.ClusterPolicyLister
	getErr  error
	listErr error
}

func (l erroringCpolLister) Get(string) (*kyvernov1.ClusterPolicy, error) {
	if l.getErr != nil {
		return nil, l.getErr
	}
	return nil, nil
}

func (l erroringCpolLister) List(labels.Selector) ([]*kyvernov1.ClusterPolicy, error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	return nil, nil
}

type erroringPolLister struct {
	kyvernov1listers.PolicyLister
	listErr error
	getErr  error
}

func (l erroringPolLister) Policies(string) kyvernov1listers.PolicyNamespaceLister {
	return erroringPolNamespaceLister{
		listErr: l.listErr,
		getErr:  l.getErr,
	}
}

type erroringPolNamespaceLister struct {
	kyvernov1listers.PolicyNamespaceLister
	listErr error
	getErr  error
}

func (l erroringPolNamespaceLister) List(labels.Selector) ([]*kyvernov1.Policy, error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	return nil, nil
}

func (l erroringPolNamespaceLister) Get(string) (*kyvernov1.Policy, error) {
	if l.getErr != nil {
		return nil, l.getErr
	}
	return nil, nil
}

func newCpolLister(cpols ...*kyvernov1.ClusterPolicy) kyvernov1listers.ClusterPolicyLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, cpol := range cpols {
		_ = indexer.Add(cpol)
	}
	return kyvernov1listers.NewClusterPolicyLister(indexer)
}

func newPolLister(pols ...*kyvernov1.Policy) kyvernov1listers.PolicyLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, pol := range pols {
		_ = indexer.Add(pol)
	}
	return kyvernov1listers.NewPolicyLister(indexer)
}

func newTestController(
	fc *fakeCache,
	cpols []*kyvernov1.ClusterPolicy,
	pols []*kyvernov1.Policy,
) *controller {
	return &controller{
		cache:      fc,
		cpolLister: newCpolLister(cpols...),
		polLister:  newPolLister(pols...),
		client:     dclient.NewEmptyFakeClient(),
	}
}

func readyClusterPolicy(name string) *kyvernov1.ClusterPolicy {
	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	cpol.Status.SetReady(true, "")
	return cpol
}

func notReadyClusterPolicy(name string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func readyPolicy(namespace, name string) *kyvernov1.Policy {
	pol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	pol.Status.SetReady(true, "")
	return pol
}

func notReadyPolicy(namespace, name string) *kyvernov1.Policy {
	return &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func TestNewController(t *testing.T) {
	t.Parallel()
	client := dclient.NewEmptyFakeClient()
	fc := &fakeCache{}

	cpol := readyClusterPolicy("cpol-test")
	pol := readyPolicy("ns-test", "pol-test")
	fakeClient := versionedfake.NewSimpleClientset(cpol, pol)
	factory := externalversions.NewSharedInformerFactory(fakeClient, 0)
	cpolInformer := factory.Kyverno().V1().ClusterPolicies()
	polInformer := factory.Kyverno().V1().Policies()

	ctrl := NewController(client, fc, cpolInformer, polInformer)
	require.NotNil(t, ctrl)
	c, ok := ctrl.(*controller)
	require.True(t, ok)
	defer c.queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	factory.Start(ctx.Done())
	require.True(t, cache.WaitForCacheSync(ctx.Done(), cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced))

	assert.Eventually(t, func() bool {
		return c.queue.Len() == 2
	}, 2*time.Second, 10*time.Millisecond)

	enqueued := make(map[any]bool)
	for c.queue.Len() > 0 {
		item, _ := c.queue.Get()
		enqueued[item] = true
		c.queue.Done(item)
	}

	assert.True(t, enqueued["cpol-test"], "cpol-test should be enqueued by clusterpolicy event handler")
	assert.True(t, enqueued["ns-test/pol-test"], "ns-test/pol-test should be enqueued by policy event handler")
}

func TestLoadPolicy(t *testing.T) {
	t.Parallel()

	cpol := readyClusterPolicy("my-cpol")
	pol := readyPolicy("test-ns", "my-pol")

	c := newTestController(&fakeCache{}, []*kyvernov1.ClusterPolicy{cpol}, []*kyvernov1.Policy{pol})

	t.Run("ClusterPolicy found", func(t *testing.T) {
		t.Parallel()
		res, err := c.loadPolicy("", "my-cpol")
		require.NoError(t, err)
		assert.Equal(t, "my-cpol", res.GetName())
	})

	t.Run("ClusterPolicy not found", func(t *testing.T) {
		t.Parallel()
		res, err := c.loadPolicy("", "non-existent")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Nil(t, res)
	})

	t.Run("Namespaced Policy found", func(t *testing.T) {
		t.Parallel()
		res, err := c.loadPolicy("test-ns", "my-pol")
		require.NoError(t, err)
		assert.Equal(t, "my-pol", res.GetName())
		assert.Equal(t, "test-ns", res.GetNamespace())
	})

	t.Run("Namespaced Policy not found", func(t *testing.T) {
		t.Parallel()
		res, err := c.loadPolicy("test-ns", "non-existent")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Nil(t, res)
	})
}

func TestReconcile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ClusterPolicy ready sets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("ready-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.reconcile(ctx, logr.Discard(), "ready-cpol", "", "ready-cpol")
		require.NoError(t, err)
		assert.Equal(t, []string{"ready-cpol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy not ready unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := notReadyClusterPolicy("not-ready-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.reconcile(ctx, logr.Discard(), "not-ready-cpol", "", "not-ready-cpol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"not-ready-cpol"}, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy not found unsets cache and returns error", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		c := newTestController(fc, nil, nil)

		err := c.reconcile(ctx, logr.Discard(), "missing-cpol", "", "missing-cpol")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"missing-cpol"}, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy generic load error does not unset cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		loadErr := errors.New("lister failure")
		c := newTestController(fc, nil, nil)
		c.cpolLister = erroringCpolLister{getErr: loadErr}

		err := c.reconcile(ctx, logr.Discard(), "cpol", "", "cpol")
		require.ErrorIs(t, err, loadErr)
		assert.Empty(t, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy admission processing disabled unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("disabled-cpol")
		disabled := false
		cpol.Spec.Admission = &disabled
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.reconcile(ctx, logr.Discard(), "disabled-cpol", "", "disabled-cpol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"disabled-cpol"}, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy with custom webhook match conditions unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("custom-cpol")
		cpol.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "custom-match",
					Expression: "true",
				},
			},
		}
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.reconcile(ctx, logr.Discard(), "custom-cpol", "", "custom-cpol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"custom-cpol"}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy ready sets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "ready-pol")
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		key := "ns-1/ready-pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "ready-pol")
		require.NoError(t, err)
		assert.Equal(t, []string{key}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy not ready unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := notReadyPolicy("ns-1", "not-ready-pol")
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		key := "ns-1/not-ready-pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "not-ready-pol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{key}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy not found unsets cache and returns error", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		c := newTestController(fc, nil, nil)

		key := "ns-1/missing-pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "missing-pol")
		require.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{key}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy generic load error does not unset cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		loadErr := errors.New("namespaced lister failure")
		c := newTestController(fc, nil, nil)
		c.polLister = erroringPolLister{getErr: loadErr}

		key := "ns-1/pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "pol")
		require.ErrorIs(t, err, loadErr)
		assert.Empty(t, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy admission processing disabled unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "disabled-pol")
		disabled := false
		pol.Spec.Admission = &disabled
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		key := "ns-1/disabled-pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "disabled-pol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{key}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy with custom webhook match conditions unsets cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "custom-pol")
		pol.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "custom-match",
					Expression: "true",
				},
			},
		}
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		key := "ns-1/custom-pol"
		err := c.reconcile(ctx, logr.Discard(), key, "ns-1", "custom-pol")
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{key}, fc.getUnsetCalls())
	})

	t.Run("Cache set error propagates error", func(t *testing.T) {
		t.Parallel()
		setErr := errors.New("failed to set cache")
		fc := &fakeCache{setErr: setErr}
		cpol := readyClusterPolicy("err-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.reconcile(ctx, logr.Discard(), "err-cpol", "", "err-cpol")
		require.Error(t, err)
		assert.Equal(t, setErr, err)
		assert.Equal(t, []string{"err-cpol"}, fc.getSetCalls())
	})
}

func TestWarmUp(t *testing.T) {
	t.Parallel()

	t.Run("Empty listers does nothing", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		c := newTestController(fc, nil, nil)

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Ready ClusterPolicy warms up into cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("ready-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Equal(t, []string{"ready-cpol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Not ready ClusterPolicy unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := notReadyClusterPolicy("not-ready-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"not-ready-cpol"}, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy admission disabled unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("disabled-cpol")
		disabled := false
		cpol.Spec.Admission = &disabled
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"disabled-cpol"}, fc.getUnsetCalls())
	})

	t.Run("ClusterPolicy custom webhook match conditions unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol := readyClusterPolicy("custom-cpol")
		cpol.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "custom-match",
					Expression: "true",
				},
			},
		}
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, nil)

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"custom-cpol"}, fc.getUnsetCalls())
	})

	t.Run("Ready Namespaced Policy warms up into cache", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "ready-pol")
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Equal(t, []string{"ns-1/ready-pol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Not ready Namespaced Policy unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := notReadyPolicy("ns-1", "not-ready-pol")
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"ns-1/not-ready-pol"}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy admission disabled unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "disabled-pol")
		disabled := false
		pol.Spec.Admission = &disabled
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"ns-1/disabled-pol"}, fc.getUnsetCalls())
	})

	t.Run("Namespaced Policy custom webhook match conditions unsets during warm up", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "custom-pol")
		pol.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "custom-match",
					Expression: "true",
				},
			},
		}
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})

		err := c.WarmUp()
		require.NoError(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Equal(t, []string{"ns-1/custom-pol"}, fc.getUnsetCalls())
	})

	t.Run("Mixed policies warm up correctly", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		cpol1 := readyClusterPolicy("ready-cpol")
		cpol2 := notReadyClusterPolicy("not-ready-cpol")
		pol1 := readyPolicy("ns-1", "ready-pol")
		pol2 := notReadyPolicy("ns-2", "not-ready-pol")

		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol1, cpol2}, []*kyvernov1.Policy{pol1, pol2})

		err := c.WarmUp()
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{"ready-cpol", "ns-1/ready-pol"}, fc.getSetCalls())
		assert.ElementsMatch(t, []string{"not-ready-cpol", "ns-2/not-ready-pol"}, fc.getUnsetCalls())
	})

	t.Run("polLister.List error during warm up returns error", func(t *testing.T) {
		t.Parallel()
		listErr := errors.New("failed to list policies")
		fc := &fakeCache{}
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{readyClusterPolicy("ready-cpol")}, nil)
		c.polLister = erroringPolLister{listErr: listErr}

		err := c.WarmUp()
		require.ErrorIs(t, err, listErr)
		assert.Empty(t, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("cpolLister.List error during warm up returns error", func(t *testing.T) {
		t.Parallel()
		listErr := errors.New("failed to list cluster policies")
		fc := &fakeCache{}
		pol := readyPolicy("ns-1", "ready-pol")
		c := newTestController(fc, nil, []*kyvernov1.Policy{pol})
		c.cpolLister = erroringCpolLister{listErr: listErr}

		err := c.WarmUp()
		require.ErrorIs(t, err, listErr)
		assert.Equal(t, []string{"ns-1/ready-pol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Cache set error on Namespaced Policy during warm up returns error and exits early", func(t *testing.T) {
		t.Parallel()
		setErr := errors.New("failed to warm up namespaced")
		fc := &fakeCache{setErr: setErr}
		pol := readyPolicy("ns-1", "ready-pol")
		cpol := readyClusterPolicy("ready-cpol")
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{cpol}, []*kyvernov1.Policy{pol})

		err := c.WarmUp()
		require.Error(t, err)
		assert.Equal(t, setErr, err)
		assert.Equal(t, []string{"ns-1/ready-pol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("Cache set error on ClusterPolicy during warm up returns error", func(t *testing.T) {
		t.Parallel()
		setErr := errors.New("failed to warm up cluster policy")
		fc := &fakeCache{setErr: setErr}
		c := newTestController(fc, []*kyvernov1.ClusterPolicy{readyClusterPolicy("ready-cpol")}, nil)

		err := c.WarmUp()
		require.Error(t, err)
		assert.Equal(t, setErr, err)
		assert.Equal(t, []string{"ready-cpol"}, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})

	t.Run("warmUpPolicy returns error when policy key extraction fails", func(t *testing.T) {
		t.Parallel()
		fc := &fakeCache{}
		c := newTestController(fc, nil, nil)

		err := c.warmUpPolicy(nil)
		require.Error(t, err)
		assert.Empty(t, fc.getSetCalls())
		assert.Empty(t, fc.getUnsetCalls())
	})
}

type erroringSharedIndexInformer struct {
	cache.SharedIndexInformer
	addErr error
}

func (e erroringSharedIndexInformer) AddEventHandler(cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return nil, e.addErr
}

type erroringClusterPolicyInformer struct {
	kyvernov1informers.ClusterPolicyInformer
	informer cache.SharedIndexInformer
}

func (e erroringClusterPolicyInformer) Informer() cache.SharedIndexInformer {
	return e.informer
}

type erroringPolicyInformer struct {
	kyvernov1informers.PolicyInformer
	informer cache.SharedIndexInformer
}

func (e erroringPolicyInformer) Informer() cache.SharedIndexInformer {
	return e.informer
}

// TestNewControllerEventHandlerError verifies that NewController still returns a valid controller
// when informer event handler registration returns an error.
func TestNewControllerEventHandlerError(t *testing.T) {
	t.Parallel()
	client := dclient.NewEmptyFakeClient()
	fc := &fakeCache{}

	fakeClient := versionedfake.NewSimpleClientset()
	factory := externalversions.NewSharedInformerFactory(fakeClient, 0)
	baseCpolInformer := factory.Kyverno().V1().ClusterPolicies()
	basePolInformer := factory.Kyverno().V1().Policies()

	regErr := errors.New("failed to register event handler")
	cpolInformer := erroringClusterPolicyInformer{
		ClusterPolicyInformer: baseCpolInformer,
		informer:              erroringSharedIndexInformer{SharedIndexInformer: baseCpolInformer.Informer(), addErr: regErr},
	}
	polInformer := erroringPolicyInformer{
		PolicyInformer: basePolInformer,
		informer:       erroringSharedIndexInformer{SharedIndexInformer: basePolInformer.Informer(), addErr: regErr},
	}

	ctrl := NewController(client, fc, cpolInformer, polInformer)
	require.NotNil(t, ctrl)
	c, ok := ctrl.(*controller)
	require.True(t, ok)
	defer c.queue.ShutDown()
}

// TestRun verifies that Run starts the worker loop, reconciles queued keys, and exits when ctx is canceled.
func TestRun(t *testing.T) {
	t.Parallel()
	fc := &fakeCache{}
	cpol := readyClusterPolicy("run-cpol")
	fakeClient := versionedfake.NewSimpleClientset(cpol)
	factory := externalversions.NewSharedInformerFactory(fakeClient, 0)
	cpolInformer := factory.Kyverno().V1().ClusterPolicies()
	polInformer := factory.Kyverno().V1().Policies()

	ctrl := NewController(dclient.NewEmptyFakeClient(), fc, cpolInformer, polInformer)
	c, ok := ctrl.(*controller)
	require.True(t, ok)
	c.cpolLister = newCpolLister(cpol)
	defer c.queue.ShutDown()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctrl.Run(ctx, 1)
	}()

	c.queue.Add("run-cpol")
	assert.Eventually(t, func() bool {
		return len(fc.getSetCalls()) == 1
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"run-cpol"}, fc.getSetCalls())

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

// testClusterPolicy builds a ClusterPolicy fixture with configurable admission, readiness, and matchConditions.
func testClusterPolicy(t *testing.T, name string, admission, ready bool, matchConditions []admissionregistrationv1.MatchCondition) *kyvernov1.ClusterPolicy {
	t.Helper()
	var policy kyvernov1.ClusterPolicy
	raw := `{
		"apiVersion":"kyverno.io/v1",
		"kind":"ClusterPolicy",
		"metadata":{"name":"` + name + `"},
		"spec":{
			"admission":` + map[bool]string{true: "true", false: "false"}[admission] + `,
			"background":true,
			"validationFailureAction":"Enforce",
			"rules":[{
				"name":"require-label",
				"match":{"any":[{"resources":{"kinds":["Pod"]}}]},
				"validate":{"message":"label required","pattern":{"metadata":{"labels":{"app":"?*"}}}}
			}]
		}
	}`
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatalf("unmarshal cluster policy %q: %v", name, err)
	}
	if len(matchConditions) > 0 {
		policy.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: matchConditions,
		}
	}
	policy.Status.SetReady(ready, "test")
	return &policy
}

// testNamespacedPolicy builds a namespaced Policy fixture with configurable admission, readiness, and matchConditions.
func testNamespacedPolicy(t *testing.T, namespace, name string, admission, ready bool, matchConditions []admissionregistrationv1.MatchCondition) *kyvernov1.Policy {
	t.Helper()
	var policy kyvernov1.Policy
	raw := `{
		"apiVersion":"kyverno.io/v1",
		"kind":"Policy",
		"metadata":{"namespace":"` + namespace + `","name":"` + name + `"},
		"spec":{
			"admission":` + map[bool]string{true: "true", false: "false"}[admission] + `,
			"background":true,
			"validationFailureAction":"Enforce",
			"rules":[{
				"name":"require-label",
				"match":{"any":[{"resources":{"kinds":["Pod"]}}]},
				"validate":{"message":"label required","pattern":{"metadata":{"labels":{"app":"?*"}}}}
			}]
		}
	}`
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		t.Fatalf("unmarshal namespaced policy %q/%q: %v", namespace, name, err)
	}
	if len(matchConditions) > 0 {
		policy.Spec.WebhookConfiguration = &kyvernov1.WebhookConfiguration{
			MatchConditions: matchConditions,
		}
	}
	policy.Status.SetReady(ready, "test")
	return &policy
}

// TestWarmUpRespectsAdmissionAndCustomWebhookMatchConditions verifies with the real policy cache
// that WarmUp and reconcile apply identical eligibility filtering and evict previously cached
// policies that become ineligible.
func TestWarmUpRespectsAdmissionAndCustomWebhookMatchConditions(t *testing.T) {
	t.Parallel()
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	matchCond := []admissionregistrationv1.MatchCondition{
		{Name: "only-prod", Expression: "object.metadata.Namespace == 'prod'"},
	}

	disabledReady := testClusterPolicy(t, "00-disabled-ready", false, true, nil)
	customMatchCondReady := testClusterPolicy(t, "01-custom-match-cond-ready", true, true, matchCond)
	enabledNotReady := testClusterPolicy(t, "02-enabled-not-ready", true, false, nil)
	enabledReadyA := testClusterPolicy(t, "03-enabled-ready-a", true, true, nil)
	enabledReadyB := testClusterPolicy(t, "04-enabled-ready-b", true, true, nil)

	cpols := []*kyvernov1.ClusterPolicy{
		disabledReady,
		customMatchCondReady,
		enabledNotReady,
		enabledReadyA,
		enabledReadyB,
	}

	nsDisabledReady := testNamespacedPolicy(t, ns.Name, "00-ns-disabled-ready", false, true, nil)
	nsCustomMatchCondReady := testNamespacedPolicy(t, ns.Name, "01-ns-custom-match-cond-ready", true, true, matchCond)
	nsEnabledNotReady := testNamespacedPolicy(t, ns.Name, "02-ns-enabled-not-ready", true, false, nil)
	nsEnabledReadyA := testNamespacedPolicy(t, ns.Name, "03-ns-enabled-ready-a", true, true, nil)
	nsEnabledReadyB := testNamespacedPolicy(t, ns.Name, "04-ns-enabled-ready-b", true, true, nil)

	pols := []*kyvernov1.Policy{
		nsDisabledReady,
		nsCustomMatchCondReady,
		nsEnabledNotReady,
		nsEnabledReadyA,
		nsEnabledReadyB,
	}

	clusterIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, cp := range cpols {
		if err := clusterIndex.Add(cp); err != nil {
			t.Fatalf("add cluster policy %q: %v", cp.Name, err)
		}
	}
	nsIndex := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	for _, p := range pols {
		if err := nsIndex.Add(p); err != nil {
			t.Fatalf("add namespaced policy %q/%q: %v", p.Namespace, p.Name, err)
		}
	}

	client := dclient.NewEmptyFakeClient()
	client.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{
		podGVR,
		{Version: "v1", Resource: "replicationcontrollers"},
		{Group: "apps", Version: "v1", Resource: "replicasets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
	}))

	policyCache := pcache.NewCache()
	ctrl := &controller{
		cache:      policyCache,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(clusterIndex),
		polLister:  kyvernov1listers.NewPolicyLister(nsIndex),
		client:     client,
	}

	if err := ctrl.WarmUp(); err != nil {
		t.Fatalf("WarmUp failed: %v", err)
	}

	got := policyCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns)
	gotNames := map[string]bool{}
	for _, p := range got {
		gotNames[p.GetName()] = true
	}
	wantInitial := map[string]bool{
		"03-enabled-ready-a":    true,
		"04-enabled-ready-b":    true,
		"03-ns-enabled-ready-a": true,
		"04-ns-enabled-ready-b": true,
	}
	if !reflect.DeepEqual(gotNames, wantInitial) {
		t.Fatalf("WarmUp cached unexpected policies: got=%v want=%v", gotNames, wantInitial)
	}

	reconcileCache := pcache.NewCache()
	reconcileCtrl := &controller{
		cache:      reconcileCache,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(clusterIndex),
		polLister:  kyvernov1listers.NewPolicyLister(nsIndex),
		client:     client,
	}
	for _, cp := range cpols {
		if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), cp.Name, "", cp.Name); err != nil {
			t.Fatalf("reconcile %q failed: %v", cp.Name, err)
		}
	}
	for _, p := range pols {
		key := p.Namespace + "/" + p.Name
		if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), key, p.Namespace, p.Name); err != nil {
			t.Fatalf("reconcile %q failed: %v", key, err)
		}
	}
	reconcileGot := reconcileCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns)
	reconcileNames := map[string]bool{}
	for _, p := range reconcileGot {
		reconcileNames[p.GetName()] = true
	}
	if !reflect.DeepEqual(gotNames, reconcileNames) {
		t.Fatalf("WarmUp and reconcile diverged: warmup=%v reconcile=%v", gotNames, reconcileNames)
	}

	// Transition previously cached policies to ineligible states and verify Unset removes them.
	admissionFalse := false
	enabledReadyB.Spec.Admission = &admissionFalse
	if err := clusterIndex.Update(enabledReadyB); err != nil {
		t.Fatalf("update cluster policy %q: %v", enabledReadyB.Name, err)
	}
	nsEnabledReadyB.Status.SetReady(false, "not-ready")
	if err := nsIndex.Update(nsEnabledReadyB); err != nil {
		t.Fatalf("update namespaced policy %q/%q: %v", nsEnabledReadyB.Namespace, nsEnabledReadyB.Name, err)
	}

	if err := ctrl.WarmUp(); err != nil {
		t.Fatalf("second WarmUp failed: %v", err)
	}
	if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), enabledReadyB.Name, "", enabledReadyB.Name); err != nil {
		t.Fatalf("reconcile updated %q failed: %v", enabledReadyB.Name, err)
	}
	nsKeyB := nsEnabledReadyB.Namespace + "/" + nsEnabledReadyB.Name
	if err := reconcileCtrl.reconcile(context.Background(), logr.Discard(), nsKeyB, nsEnabledReadyB.Namespace, nsEnabledReadyB.Name); err != nil {
		t.Fatalf("reconcile updated %q failed: %v", nsKeyB, err)
	}

	afterWarmUpNames := map[string]bool{}
	for _, p := range policyCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns) {
		afterWarmUpNames[p.GetName()] = true
	}
	afterReconcileNames := map[string]bool{}
	for _, p := range reconcileCache.GetPolicies(pcache.ValidateEnforce, podGVR, "", ns) {
		afterReconcileNames[p.GetName()] = true
	}
	wantAfterEviction := map[string]bool{
		"03-enabled-ready-a":    true,
		"03-ns-enabled-ready-a": true,
	}
	if !reflect.DeepEqual(afterWarmUpNames, wantAfterEviction) {
		t.Fatalf("WarmUp did not evict ineligible policies: got=%v want=%v", afterWarmUpNames, wantAfterEviction)
	}
	if !reflect.DeepEqual(afterWarmUpNames, afterReconcileNames) {
		t.Fatalf("WarmUp and reconcile diverged after eviction: warmup=%v reconcile=%v", afterWarmUpNames, afterReconcileNames)
	}
}
