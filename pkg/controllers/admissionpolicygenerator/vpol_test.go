package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

func TestEnqueueVP_ClusterValidatingPolicy(t *testing.T) {
	t.Parallel()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	defer queue.ShutDown()

	c := &controller{queue: queue}

	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vpol",
		},
	}

	c.enqueueVP(vpol)

	assert.Equal(t, 1, queue.Len(), "ValidatingPolicy must be enqueued")
	item, _ := queue.Get()
	defer queue.Done(item)
	assert.Equal(t, "ValidatingPolicy/test-vpol", item, "queue key must have ValidatingPolicy/ prefix")
}

func TestEnqueueVP_NamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	defer queue.ShutDown()

	c := &controller{queue: queue}

	nvpol := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-nvpol",
			Namespace: "test-namespace",
		},
	}

	c.enqueueVP(nvpol)

	assert.Equal(t, 1, queue.Len(), "NamespacedValidatingPolicy must be enqueued")
	item, _ := queue.Get()
	defer queue.Done(item)
	assert.Equal(t, "NamespacedValidatingPolicy/test-namespace/test-nvpol", item, "queue key must have NamespacedValidatingPolicy/ prefix and namespace/name")
}

func TestHandlersVP_NamespacedValidatingPolicy(t *testing.T) {
	t.Parallel()
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
	defer queue.ShutDown()

	c := &controller{queue: queue}

	nvpolOld := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-nvpol",
			Namespace: "default",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []policiesv1beta1.Validation{
				{Message: "msg1"},
			},
		},
	}
	nvpolNew := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-nvpol",
			Namespace: "default",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			Validations: []policiesv1beta1.Validation{
				{Message: "msg2"},
			},
		},
	}

	c.addVP(nvpolOld)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	queue.Done(item)
	assert.Equal(t, "NamespacedValidatingPolicy/default/test-nvpol", item)

	c.updateVP(nvpolOld, nvpolNew)
	assert.Equal(t, 1, queue.Len())
	item, _ = queue.Get()
	queue.Done(item)
	assert.Equal(t, "NamespacedValidatingPolicy/default/test-nvpol", item)

	c.deleteVP(nvpolNew)
	assert.Equal(t, 1, queue.Len())
	item, _ = queue.Get()
	queue.Done(item)
	assert.Equal(t, "NamespacedValidatingPolicy/default/test-nvpol", item)
}
