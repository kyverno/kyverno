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
}

func TestDeleteFunc(t *testing.T) {
	enqueuer := &mockEnqueuer{}
	deleteFunc := DeleteFunc(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	deleteFunc(obj)
	assert.Equal(t, []interface{}{obj}, enqueuer.enqueued)
}

func TestAddFuncT(t *testing.T) {
	enqueuer := &mockTypedEnqueuer[*metav1.ObjectMeta]{}
	addFunc := AddFuncT(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	addFunc(obj)
	assert.Equal(t, []*metav1.ObjectMeta{obj}, enqueuer.enqueued)
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
}

func TestDeleteFuncT(t *testing.T) {
	enqueuer := &mockTypedEnqueuer[*metav1.ObjectMeta]{}
	deleteFunc := DeleteFuncT(logr.Discard(), enqueuer.Enqueue)
	obj := &metav1.ObjectMeta{ResourceVersion: "1"}
	deleteFunc(obj)
	assert.Equal(t, []*metav1.ObjectMeta{obj}, enqueuer.enqueued)
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
}

func TestQueue(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	enqueue := Queue(queue)
	err := enqueue("foo")
	assert.NoError(t, err)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "foo", item)
}

func TestQueueAfter(t *testing.T) {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	enqueue := QueueAfter(queue, 10*time.Millisecond)
	err := enqueue("foo")
	assert.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, queue.Len())
	item, _ := queue.Get()
	assert.Equal(t, "foo", item)
}

func TestMetaNamespaceKey(t *testing.T) {
	obj := &metav1.ObjectMeta{Name: "foo", Namespace: "bar"}
	key, err := MetaNamespaceKey(obj)
	assert.NoError(t, err)
	expected, err := cache.MetaNamespaceKeyFunc(obj)
	assert.NoError(t, err)
	assert.Equal(t, expected, key)
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
