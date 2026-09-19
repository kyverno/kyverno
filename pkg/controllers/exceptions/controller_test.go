package exceptions

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func newTestQueue() workqueue.TypedRateLimitingInterface[any] {
	return workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-queue"},
	)
}

func newNamespaceIndexer() cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
}

func drainQueue(q workqueue.TypedRateLimitingInterface[any]) []string {
	var items []string
	for q.Len() > 0 {
		item, _ := q.Get()
		if s, ok := item.(string); ok {
			items = append(items, s)
		}
		q.Done(item)
	}
	return items
}

func TestFind_EmptyIndex(t *testing.T) {
	t.Parallel()

	c := &controller{
		index: policyIndex{},
	}

	res, err := c.Find("policy1", "rule1")
	assert.NoError(t, err)
	assert.Nil(t, res)
}

func TestFind_HitByPolicyAndRule(t *testing.T) {
	t.Parallel()

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-polex",
		},
	}
	c := &controller{
		index: policyIndex{
			"policy1": {
				"rule1": []*kyvernov2.PolicyException{polex},
			},
		},
	}

	res, err := c.Find("policy1", "rule1")
	assert.NoError(t, err)
	assert.Equal(t, []*kyvernov2.PolicyException{polex}, res)
}

func TestFind_MissOnRule(t *testing.T) {
	t.Parallel()

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-polex",
		},
	}
	c := &controller{
		index: policyIndex{
			"policy1": {
				"rule1": []*kyvernov2.PolicyException{polex},
			},
		},
	}

	res, err := c.Find("policy1", "rule2")
	assert.NoError(t, err)
	assert.Nil(t, res)
}

func TestFind_MissOnPolicy(t *testing.T) {
	t.Parallel()

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-polex",
		},
	}
	c := &controller{
		index: policyIndex{
			"policy1": {
				"rule1": []*kyvernov2.PolicyException{polex},
			},
		},
	}

	res, err := c.Find("policy2", "rule1")
	assert.NoError(t, err)
	assert.Nil(t, res)
}

func TestAddPolex(t *testing.T) {
	t.Parallel()

	q := newTestQueue()
	defer q.ShutDown()

	c := &controller{queue: q}

	polex := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "pol-1"},
				{PolicyName: "pol-2"},
				{PolicyName: "pol-1"},
			},
		},
	}

	c.addPolex(polex)

	items := drainQueue(q)
	assert.ElementsMatch(t, []string{"pol-1", "pol-2"}, items)
}

func TestUpdatePolex(t *testing.T) {
	t.Parallel()

	q := newTestQueue()
	defer q.ShutDown()

	c := &controller{queue: q}

	oldPolex := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "pol-1"},
				{PolicyName: "pol-2"},
			},
		},
	}
	newPolex := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "pol-2"},
				{PolicyName: "pol-3"},
			},
		},
	}

	c.updatePolex(oldPolex, newPolex)

	items := drainQueue(q)
	assert.ElementsMatch(t, []string{"pol-1", "pol-2", "pol-3"}, items)
}

func TestDeletePolex(t *testing.T) {
	t.Parallel()

	q := newTestQueue()
	defer q.ShutDown()

	c := &controller{queue: q}

	polex := &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "pol-1"},
				{PolicyName: "pol-2"},
			},
		},
	}

	c.deletePolex(polex)

	items := drainQueue(q)
	assert.ElementsMatch(t, []string{"pol-1", "pol-2"}, items)
}

func TestReconcile_ClusterPolicy(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpol-1",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{Name: "rule-1"},
				{Name: "rule-2"},
			},
		},
	}
	require.NoError(t, cpolIndexer.Add(cpol))

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polex-1",
			Namespace: "default",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "cpol-1",
					RuleNames:  []string{"rule-1"},
				},
			},
		},
	}
	require.NoError(t, polexIndexer.Add(polex))

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index:       policyIndex{},
		namespace:   "*",
	}

	err := c.reconcile(context.Background(), logr.Discard(), "cpol-1", "", "cpol-1")
	assert.NoError(t, err)

	rule1Exceptions, err := c.Find("cpol-1", "rule-1")
	assert.NoError(t, err)
	require.Len(t, rule1Exceptions, 1)
	assert.Equal(t, "polex-1", rule1Exceptions[0].Name)

	rule2Exceptions, err := c.Find("cpol-1", "rule-2")
	assert.NoError(t, err)
	assert.Empty(t, rule2Exceptions)
}

func TestReconcile_NamespacedPolicy(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	pol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns1",
			Name:      "pol-1",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{Name: "rule-1"},
			},
		},
	}
	require.NoError(t, polIndexer.Add(pol))

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polex-1",
			Namespace: "ns1",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "ns1/pol-1",
					RuleNames:  []string{"rule-1"},
				},
			},
		},
	}
	require.NoError(t, polexIndexer.Add(polex))

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index:       policyIndex{},
		namespace:   "*",
	}

	err := c.reconcile(context.Background(), logr.Discard(), "ns1/pol-1", "ns1", "pol-1")
	assert.NoError(t, err)

	res, err := c.Find("ns1/pol-1", "rule-1")
	assert.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "polex-1", res[0].Name)
}

func TestReconcile_NotFoundDeletesIndex(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index: policyIndex{
			"cpol-1": {
				"rule-1": []*kyvernov2.PolicyException{
					{ObjectMeta: metav1.ObjectMeta{Name: "polex-1"}},
				},
			},
		},
		namespace: "*",
	}

	err := c.reconcile(context.Background(), logr.Discard(), "cpol-1", "", "cpol-1")
	assert.NoError(t, err)

	res, err := c.Find("cpol-1", "rule-1")
	assert.NoError(t, err)
	assert.Nil(t, res)

	c.lock.RLock()
	_, exists := c.index["cpol-1"]
	c.lock.RUnlock()
	assert.False(t, exists)
}

func TestReconcile_NamespaceScopedExceptions(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpol-1",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{Name: "rule-1"},
			},
		},
	}
	require.NoError(t, cpolIndexer.Add(cpol))

	polex1 := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polex-ns1",
			Namespace: "ns1",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "cpol-1",
					RuleNames:  []string{"rule-1"},
				},
			},
		},
	}
	polex2 := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polex-ns2",
			Namespace: "ns2",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{
					PolicyName: "cpol-1",
					RuleNames:  []string{"rule-1"},
				},
			},
		},
	}
	require.NoError(t, polexIndexer.Add(polex1))
	require.NoError(t, polexIndexer.Add(polex2))

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index:       policyIndex{},
		namespace:   "ns1",
	}

	err := c.reconcile(context.Background(), logr.Discard(), "cpol-1", "", "cpol-1")
	assert.NoError(t, err)

	res, err := c.Find("cpol-1", "rule-1")
	assert.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "polex-ns1", res[0].Name)
	assert.Equal(t, "ns1", res[0].Namespace)
}

func TestReconcile_DeterministicOrdering(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cpol-1",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{Name: "rule-1"},
			},
		},
	}
	require.NoError(t, cpolIndexer.Add(cpol))

	polexB := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "b-polex",
			Namespace: "ns2",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}},
			},
		},
	}
	polexA := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-polex",
			Namespace: "ns2",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}},
			},
		},
	}
	polexNS1 := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "z-polex",
			Namespace: "ns1",
		},
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "cpol-1", RuleNames: []string{"rule-1"}},
			},
		},
	}

	require.NoError(t, polexIndexer.Add(polexB))
	require.NoError(t, polexIndexer.Add(polexA))
	require.NoError(t, polexIndexer.Add(polexNS1))

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index:       policyIndex{},
		namespace:   "*",
	}

	err := c.reconcile(context.Background(), logr.Discard(), "cpol-1", "", "cpol-1")
	assert.NoError(t, err)

	res, err := c.Find("cpol-1", "rule-1")
	assert.NoError(t, err)
	require.Len(t, res, 3)
	assert.Equal(t, "ns1", res[0].Namespace)
	assert.Equal(t, "z-polex", res[0].Name)
	assert.Equal(t, "ns2", res[1].Namespace)
	assert.Equal(t, "a-polex", res[1].Name)
	assert.Equal(t, "ns2", res[2].Namespace)
	assert.Equal(t, "b-polex", res[2].Name)
}

func TestFind_ConcurrentReads(t *testing.T) {
	t.Parallel()

	polex := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-polex",
		},
	}
	c := &controller{
		index: policyIndex{
			"policy1": {
				"rule1": []*kyvernov2.PolicyException{polex},
			},
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				res, err := c.Find("policy1", "rule1")
				assert.NoError(t, err)
				assert.Len(t, res, 1)
			}
		}()
	}
	wg.Wait()
}

func TestReconcile_ConcurrentWithFind(t *testing.T) {
	t.Parallel()

	cpolIndexer := newNamespaceIndexer()
	polIndexer := newNamespaceIndexer()
	polexIndexer := newNamespaceIndexer()

	for i := 0; i < 10; i++ {
		cpol := &kyvernov1.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("policy-%d", i),
			},
			Spec: kyvernov1.Spec{
				Rules: []kyvernov1.Rule{
					{Name: "rule-1"},
				},
			},
		}
		require.NoError(t, cpolIndexer.Add(cpol))
	}

	c := &controller{
		cpolLister:  kyvernov1listers.NewClusterPolicyLister(cpolIndexer),
		polLister:   kyvernov1listers.NewPolicyLister(polIndexer),
		polexLister: kyvernov2listers.NewPolicyExceptionLister(polexIndexer),
		index:       policyIndex{},
		namespace:   "*",
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		policyName := fmt.Sprintf("policy-%d", i)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = c.reconcile(ctx, logr.Discard(), policyName, "", policyName)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		policyName := fmt.Sprintf("policy-%d", i)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = c.Find(policyName, "rule-1")
			}
		}()
	}

	wg.Wait()
}
