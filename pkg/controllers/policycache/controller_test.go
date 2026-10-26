package policycache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions"
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
}
