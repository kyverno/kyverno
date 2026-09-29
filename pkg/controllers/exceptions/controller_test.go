package exceptions

import (
	"sort"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
)

func makeCpol(name string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func makePol(namespace, name string) *kyvernov1.Policy {
	return &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

func newTestController(t *testing.T, objects ...runtime.Object) (*controller, func()) {
	t.Helper()
	client := versionedfake.NewSimpleClientset(objects...)
	factory := kyvernoinformer.NewSharedInformerFactory(client, 0)
	cpolInformer := factory.Kyverno().V1().ClusterPolicies()
	polInformer := factory.Kyverno().V1().Policies()
	polexInformer := factory.Kyverno().V2().PolicyExceptions()

	stopCh := make(chan struct{})
	c := NewController(cpolInformer, polInformer, polexInformer, "*")
	factory.Start(stopCh)
	// Assert that all informer caches have synced before proceeding.
	synced := factory.WaitForCacheSync(stopCh)
	for informerType, ok := range synced {
		require.Truef(t, ok, "informer %v failed to sync", informerType)
	}
	// Wait briefly for any pending event handler deliveries after cache sync.
	time.Sleep(50 * time.Millisecond)
	// Drain the initial Add events that the informer fires for every existing
	// object — those are unrelated to the wildcard expansion under test.
	for c.queue.Len() > 0 {
		key, _ := c.queue.Get()
		c.queue.Done(key)
		c.queue.Forget(key)
	}
	return c, func() { close(stopCh) }
}

func drainQueue(q workqueue.TypedRateLimitingInterface[any]) []string {
	out := []string{}
	for q.Len() > 0 {
		key, _ := q.Get()
		out = append(out, key.(string))
		q.Done(key)
		q.Forget(key)
	}
	sort.Strings(out)
	return out
}

func TestEnqueuePoliciesForExceptions_Literal(t *testing.T) {
	c, stop := newTestController(t,
		makeCpol("disallow-foo"),
		makeCpol("disallow-bar"),
		makePol("default", "require-labels"),
	)
	defer stop()

	c.enqueuePoliciesForExceptions([]kyvernov2.Exception{
		{PolicyName: "disallow-foo"},
		{PolicyName: "default/require-labels"},
	})

	assert.Equal(t, []string{"default/require-labels", "disallow-foo"}, drainQueue(c.queue))
}

func TestEnqueuePoliciesForExceptions_WildcardMatchAll(t *testing.T) {
	c, stop := newTestController(t,
		makeCpol("disallow-foo"),
		makeCpol("disallow-bar"),
		makePol("default", "require-labels"),
		makePol("kube-system", "no-priv"),
	)
	defer stop()

	c.enqueuePoliciesForExceptions([]kyvernov2.Exception{{PolicyName: "*"}})

	assert.Equal(t, []string{
		"default/require-labels",
		"disallow-bar",
		"disallow-foo",
		"kube-system/no-priv",
	}, drainQueue(c.queue))
}

func TestEnqueuePoliciesForExceptions_WildcardPrefixCpol(t *testing.T) {
	c, stop := newTestController(t,
		makeCpol("disallow-foo"),
		makeCpol("disallow-bar"),
		makeCpol("require-labels"),
		makePol("default", "disallow-x"),
	)
	defer stop()

	c.enqueuePoliciesForExceptions([]kyvernov2.Exception{{PolicyName: "disallow-*"}})

	assert.Equal(t, []string{"disallow-bar", "disallow-foo"}, drainQueue(c.queue))
}

func TestEnqueuePoliciesForExceptions_WildcardNamespaceScoped(t *testing.T) {
	c, stop := newTestController(t,
		makeCpol("disallow-foo"),
		makePol("default", "require-labels"),
		makePol("default", "no-priv"),
		makePol("kube-system", "kube-policy"),
	)
	defer stop()

	c.enqueuePoliciesForExceptions([]kyvernov2.Exception{{PolicyName: "default/*"}})

	assert.Equal(t, []string{"default/no-priv", "default/require-labels"}, drainQueue(c.queue))
}

func TestEnqueuePoliciesForExceptions_MixedLiteralAndWildcard(t *testing.T) {
	c, stop := newTestController(t,
		makeCpol("disallow-foo"),
		makeCpol("disallow-bar"),
		makePol("default", "require-labels"),
	)
	defer stop()

	c.enqueuePoliciesForExceptions([]kyvernov2.Exception{
		{PolicyName: "disallow-*"},
		{PolicyName: "default/require-labels"},
	})

	assert.Equal(t, []string{
		"default/require-labels",
		"disallow-bar",
		"disallow-foo",
	}, drainQueue(c.queue))
}
