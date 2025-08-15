package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"
)

// mock workqueue
type mockWorkqueue[T comparable] struct {
	workqueue.TypedRateLimitingInterface[T]
	forgotten   []T
	rateLimited []T
	numRequeues int
}

func (m *mockWorkqueue[T]) Forget(obj T) {
	m.forgotten = append(m.forgotten, obj)
}

func (m *mockWorkqueue[T]) AddRateLimited(obj T) {
	m.rateLimited = append(m.rateLimited, obj)
}

func (m *mockWorkqueue[T]) NumRequeues(obj T) int {
	return m.numRequeues
}

func newTestMetrics() *controllerMetrics {
	meter := otel.GetMeterProvider().Meter("test")
	reconcileTotal, _ := meter.Int64Counter("reconcile_total")
	requeueTotal, _ := meter.Int64Counter("requeue_total")
	queueDropTotal, _ := meter.Int64Counter("queue_drop_total")
	return &controllerMetrics{
		controllerName: "test",
		reconcileTotal: reconcileTotal,
		requeueTotal:   requeueTotal,
		queueDropTotal: queueDropTotal,
	}
}

func TestHandleErr(t *testing.T) {
	metrics := newTestMetrics()
	// no error
	t.Run("no error", func(t *testing.T) {
		queue := &mockWorkqueue[string]{}
		handleErr(context.Background(), logr.Discard(), metrics, queue, 5, nil, "foo")
		assert.Equal(t, []string{"foo"}, queue.forgotten)
		assert.Empty(t, queue.rateLimited)
	})
	// not found error
	t.Run("not found error", func(t *testing.T) {
		queue := &mockWorkqueue[string]{}
		handleErr(context.Background(), logr.Discard(), metrics, queue, 5, k8errors.NewNotFound(schema.GroupResource{}, ""), "foo")
		assert.Equal(t, []string{"foo"}, queue.forgotten)
		assert.Empty(t, queue.rateLimited)
	})
	// max retries
	t.Run("max retries", func(t *testing.T) {
		queue := &mockWorkqueue[string]{numRequeues: 5}
		handleErr(context.Background(), logr.Discard(), metrics, queue, 5, errors.New("some error"), "foo")
		assert.Equal(t, []string{"foo"}, queue.forgotten)
		assert.Empty(t, queue.rateLimited)
	})
	// retry
	t.Run("retry", func(t *testing.T) {
		queue := &mockWorkqueue[string]{numRequeues: 4}
		handleErr(context.Background(), logr.Discard(), metrics, queue, 5, errors.New("some error"), "foo")
		assert.Empty(t, queue.forgotten)
		assert.Equal(t, []string{"foo"}, queue.rateLimited)
	})
}
