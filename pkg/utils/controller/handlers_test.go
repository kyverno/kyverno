package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type mockEnqueuer struct {
	enqueued []interface{}
}

func (e *mockEnqueuer) Enqueue(obj interface{}) error {
	e.enqueued = append(e.enqueued, obj)
	return nil
}

type mockTypedEnqueuer[T any] struct {
	enqueued []T
}

func (e *mockTypedEnqueuer[T]) Enqueue(obj T) error {
	e.enqueued = append(e.enqueued, obj)
	return nil
}

func TestAddFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	addFunc := AddFunc(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	addFunc(obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)

	errFunc := AddFunc(logr.Discard(), func(interface{}) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(obj) })
}

func TestUpdateFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	updateFunc := UpdateFunc(logr.Discard(), enqueuer.Enqueue)

	// same resource version
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, &metav1.ObjectMeta{ResourceVersion: "1"})
	assert.Empty(t, enqueuer.enqueued)

	// different resource version
	obj := &metav1.ObjectMeta{ResourceVersion: "2"}
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)

	errFunc := UpdateFunc(logr.Discard(), func(interface{}) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, obj) })
}

func TestDeleteFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	deleteFunc := DeleteFunc(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	deleteFunc(obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)

	errFunc := DeleteFunc(logr.Discard(), func(interface{}) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(obj) })
}

func TestAddFuncT(t *testing.T) {
	enqueuer := &mockTypedEnqueuer[*metav1.ObjectMeta]{}
	addFunc := AddFuncT(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	addFunc(obj)
	assert.Equal(t, []*metav1.ObjectMeta{obj}, enqueuer.enqueued)

	errFunc := AddFuncT[*metav1.ObjectMeta](logr.Discard(), func(*metav1.ObjectMeta) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(obj) })
}

func TestUpdateFuncT(t *testing.T) {
	enqueuer := &mockTypedEnqueuer[*metav1.ObjectMeta]{}
	updateFunc := UpdateFuncT(logr.Discard(), enqueuer.Enqueue)

	// same resource version
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, &metav1.ObjectMeta{ResourceVersion: "1"})
	assert.Empty(t, enqueuer.enqueued)

	// different resource version
	obj := &metav1.ObjectMeta{ResourceVersion: "2"}
	updateFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, obj)
	assert.Equal(t, []*metav1.ObjectMeta{obj}, enqueuer.enqueued)

	errFunc := UpdateFuncT[*metav1.ObjectMeta](logr.Discard(), func(*metav1.ObjectMeta) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(&metav1.ObjectMeta{ResourceVersion: "1"}, obj) })
}

func TestDeleteFuncT(t *testing.T) {
	enqueuer := &mockTypedEnqueuer[*metav1.ObjectMeta]{}
	deleteFunc := DeleteFuncT(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	deleteFunc(obj)
	assert.Equal(t, []*metav1.ObjectMeta{obj}, enqueuer.enqueued)

	errFunc := DeleteFuncT[*metav1.ObjectMeta](logr.Discard(), func(*metav1.ObjectMeta) error { return errors.New("err") })
	assert.NotPanics(t, func() { errFunc(obj) })
}

func TestLogError(t *testing.T) {
	// no error
	enqueuer := &mockEnqueuer{}
	logError := LogError(logr.Discard(), enqueuer.Enqueue)
	err := logError("foo")
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"foo"}, enqueuer.enqueued)

	// with error
	logError = LogError(logr.Discard(), func(i interface{}) error {
		return errors.New("test error")
	})
	err = logError("foo")
	assert.Error(t, err)
}

func TestParse(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	keyFunc := func(obj interface{}) (interface{}, error) {
		return obj.(string) + "-key", nil
	}
	parse := Parse(keyFunc, enqueuer.Enqueue)
	err := parse("foo")
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"foo-key"}, enqueuer.enqueued)

	// error case
	errKeyFunc := func(obj interface{}) (interface{}, error) {
		return nil, errors.New("key error")
	}
	errParse := Parse(errKeyFunc, enqueuer.Enqueue)
	err = errParse("foo")
	assert.Error(t, err)
}

func TestQueue(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	enqueue := Queue(queue)
	err := enqueue("foo")
	assert.NoError(t, err)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "foo", item)
}

type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	item  any
	delay time.Duration
}

func (q *recordingQueue) AddAfter(item any, delay time.Duration) {
	q.item = item
	q.delay = delay
	q.TypedRateLimitingInterface.AddAfter(item, delay)
}

func TestQueueAfter(t *testing.T) {
	rawQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(rawQueue.ShutDown)
	queue := &recordingQueue{TypedRateLimitingInterface: rawQueue}
	delay := 10 * time.Millisecond
	enqueue := QueueAfter(queue, delay)
	err := enqueue("foo")
	assert.NoError(t, err)
	assert.Equal(t, "foo", queue.item)
	assert.Equal(t, delay, queue.delay)
	assert.Eventually(t, func() bool {
		return queue.Len() == 1
	}, 2*time.Second, 10*time.Millisecond)
	item, _ := queue.Get()
	assert.Equal(t, "foo", item)
}

func TestMetaObjectToName(t *testing.T) {
	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar"}
	assert.Equal(t, "bar/foo", MetaObjectToName(obj))

	clusterObj := &metav1.ObjectMeta{Name: "cluster-foo"}
	assert.Equal(t, "cluster-foo", MetaObjectToName(clusterObj))
}

func TestMetaNamespaceKey(t *testing.T) {
	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar"}
	key, err := MetaNamespaceKey(obj)
	assert.NoError(t, err)
	expected, err := cache.MetaNamespaceKeyFunc(obj)
	assert.NoError(t, err)
	assert.Equal(t, expected, key)
}

func TestMetaNamespaceKeyT(t *testing.T) {
	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar"}
	key, err := MetaNamespaceKeyT(obj)
	assert.NoError(t, err)
	assert.Equal(t, "bar/foo", key)
}

func TestExplicitKey(t *testing.T) {
	keyFunc := func(s string) cache.ExplicitKey {
		return cache.ExplicitKey(s)
	}
	explicitKeyFunc := ExplicitKey(keyFunc)

	// good case
	key, err := explicitKeyFunc("foo")
	assert.NoError(t, err)
	assert.Equal(t, cache.ExplicitKey("foo"), key)

	// nil object
	_, err = explicitKeyFunc(nil)
	assert.Error(t, err)

	// wrong type
	_, err = explicitKeyFunc(123)
	assert.Error(t, err)
}

type mockInformer struct {
	cache.SharedInformer
	handler cache.ResourceEventHandler
	err     error
}

func (m *mockInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.handler = handler
	return nil, nil
}

func TestAddEventHandlers(t *testing.T) {
	var added, updated, deleted interface{}
	informer := &mockInformer{}

	_, err := AddEventHandlers(
		informer,
		func(obj interface{}) { added = obj },
		func(old, obj interface{}) { updated = obj },
		func(obj interface{}) { deleted = obj },
	)
	assert.NoError(t, err)
	assert.NotNil(t, informer.handler)

	obj := &metav1.ObjectMeta{Name: "foo"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, obj, added)

	newObj := &metav1.ObjectMeta{Name: "foo", ResourceVersion: "2"}
	informer.handler.OnUpdate(obj, newObj)
	assert.Equal(t, newObj, updated)

	informer.handler.OnDelete(obj)
	assert.Equal(t, obj, deleted)

	// tombstone deletion
	tombstone := cache.DeletedFinalStateUnknown{Key: "foo/bar", Obj: obj}
	informer.handler.OnDelete(tombstone)
	assert.Equal(t, obj, deleted)

	// informer error
	informerErr := &mockInformer{err: errors.New("informer error")}
	_, err = AddEventHandlers(informerErr, nil, nil, nil)
	assert.Error(t, err)
}

func TestAddEventHandlersT(t *testing.T) {
	var added, updated, deleted *metav1.ObjectMeta
	informer := &mockInformer{}

	_, err := AddEventHandlersT[*metav1.ObjectMeta](
		informer,
		func(obj *metav1.ObjectMeta) { added = obj },
		func(old, obj *metav1.ObjectMeta) { updated = obj },
		func(obj *metav1.ObjectMeta) { deleted = obj },
	)
	assert.NoError(t, err)
	assert.NotNil(t, informer.handler)

	obj := &metav1.ObjectMeta{Name: "foo"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, obj, added)

	newObj := &metav1.ObjectMeta{Name: "foo", ResourceVersion: "2"}
	informer.handler.OnUpdate(obj, newObj)
	assert.Equal(t, newObj, updated)

	informer.handler.OnDelete(obj)
	assert.Equal(t, obj, deleted)
}

func TestAddKeyedEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddKeyedEventHandlers(logr.Discard(), informer, queue, MetaNamespaceKey)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "test-name", Namespace: "test-ns", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "test-ns/test-name", item)

	// error case
	informerErr := &mockInformer{err: errors.New("informer error")}
	_, _, err = AddKeyedEventHandlers(logr.Discard(), informerErr, queue, MetaNamespaceKey)
	assert.Error(t, err)
}

func TestAddKeyedEventHandlersT(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddKeyedEventHandlersT[*metav1.ObjectMeta](logr.Discard(), informer, queue, MetaNamespaceKeyT[*metav1.ObjectMeta])
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "test-name", Namespace: "test-ns", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "test-ns/test-name", item)

	// error case
	informerErr := &mockInformer{err: errors.New("informer error")}
	_, _, err = AddKeyedEventHandlersT[*metav1.ObjectMeta](logr.Discard(), informerErr, queue, MetaNamespaceKeyT[*metav1.ObjectMeta])
	assert.Error(t, err)
}

func TestAddDelayedKeyedEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddDelayedKeyedEventHandlers(logr.Discard(), informer, queue, 10*time.Millisecond, MetaNamespaceKey)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "delayed-name", Namespace: "delayed-ns", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Eventually(t, func() bool {
		return queue.Len() == 1
	}, 2*time.Second, 10*time.Millisecond)
	item, _ := queue.Get()
	assert.Equal(t, "delayed-ns/delayed-name", item)

	// error case
	informerErr := &mockInformer{err: errors.New("informer error")}
	_, _, err = AddDelayedKeyedEventHandlers(logr.Discard(), informerErr, queue, 10*time.Millisecond, MetaNamespaceKey)
	assert.Error(t, err)
}

func TestAddDefaultEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddDefaultEventHandlers(logr.Discard(), informer, queue)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "bar/foo", item)
}

func TestAddDefaultEventHandlersT(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddDefaultEventHandlersT[*metav1.ObjectMeta](logr.Discard(), informer, queue)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "bar/foo", item)
}

func TestAddDelayedDefaultEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	enqueueFunc, _, err := AddDelayedDefaultEventHandlers(logr.Discard(), informer, queue, 10*time.Millisecond)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar", ResourceVersion: "1"}
	informer.handler.OnAdd(obj, false)
	assert.Eventually(t, func() bool {
		return queue.Len() == 1
	}, 2*time.Second, 10*time.Millisecond)
	item, _ := queue.Get()
	assert.Equal(t, "bar/foo", item)
}

func TestAddExplicitEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	parseKey := func(s string) cache.ExplicitKey {
		return cache.ExplicitKey("custom-" + s)
	}
	enqueueFunc, _, err := AddExplicitEventHandlers(logr.Discard(), informer, queue, parseKey)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	err = enqueueFunc("key1")
	assert.NoError(t, err)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, cache.ExplicitKey("custom-key1"), item)
}

func TestAddDelayedExplicitEventHandlers(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	t.Cleanup(queue.ShutDown)
	informer := &mockInformer{}

	parseKey := func(s string) cache.ExplicitKey {
		return cache.ExplicitKey("delayed-custom-" + s)
	}
	enqueueFunc, _, err := AddDelayedExplicitEventHandlers(logr.Discard(), informer, queue, 10*time.Millisecond, parseKey)
	assert.NoError(t, err)
	assert.NotNil(t, enqueueFunc)

	err = enqueueFunc("key2")
	assert.NoError(t, err)
	assert.Eventually(t, func() bool {
		return queue.Len() == 1
	}, 2*time.Second, 10*time.Millisecond)
	item, _ := queue.Get()
	assert.Equal(t, cache.ExplicitKey("delayed-custom-key2"), item)
}
