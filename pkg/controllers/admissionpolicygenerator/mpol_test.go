package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

// TestEnqueueMP_SkipsNamespacedMutatingPolicy verifies that a NamespacedMutatingPolicy
// is NOT enqueued by enqueueMP. If it were, the key would be
// "MutatingPolicy/<namespace>/<name>" — a 3-part key that
// cache.SplitMetaNamespaceKey cannot parse, causing a permanent retry loop.
func TestEnqueueMP_SkipsNamespacedMutatingPolicy(t *testing.T) {
	t.Parallel()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	defer queue.ShutDown()

	c := &controller{queue: queue}

	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "test-namespace",
		},
	}

	c.enqueueMP(nmpol)

	assert.Equal(t, 0, queue.Len(), "NamespacedMutatingPolicy must not be enqueued by enqueueMP")
}

// TestEnqueueMP_EnqueuesMutatingPolicy verifies that a cluster-scoped MutatingPolicy
// IS enqueued with the expected "MutatingPolicy/<name>" key.
func TestEnqueueMP_EnqueuesMutatingPolicy(t *testing.T) {
	t.Parallel()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	defer queue.ShutDown()

	c := &controller{queue: queue}

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}

	c.enqueueMP(mpol)

	assert.Equal(t, 1, queue.Len(), "MutatingPolicy must be enqueued")
	item, _ := queue.Get()
	defer queue.Done(item)
	assert.Equal(t, "MutatingPolicy/test-policy", item, "queue key must have the correct prefix and name")
}
