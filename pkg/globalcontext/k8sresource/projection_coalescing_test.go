package k8sresource

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNotifyProjectionChangeCoalescesBurst(t *testing.T) {
	changes := make(chan struct{}, 1)
	for i := 0; i < 1000; i++ {
		notifyProjectionChange(changes)
	}
	require.Len(t, changes, 1)
	<-changes
	require.Empty(t, changes)
}

func TestRunProjectionWorkerCoalescesAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	changes := make(chan struct{}, 1)
	var calls atomic.Int32
	first := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		runProjectionWorker(ctx, changes, func() {
			if calls.Add(1) == 1 {
				close(first)
			}
		})
	}()
	for i := 0; i < 1000; i++ {
		notifyProjectionChange(changes)
	}
	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not refresh the projection")
	}
	// All initial changes fit into a single pending notification.
	require.EqualValues(t, 1, calls.Load())
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("projection worker failed to stop after cancellation")
	}
}

func TestRunProjectionWorkerRetainsChangesDuringRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := make(chan struct{}, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	second := make(chan struct{})
	done := make(chan struct{})
	var count atomic.Int32
	go func() {
		defer close(done)
		runProjectionWorker(ctx, changes, func() {
			switch count.Add(1) {
			case 1:
				close(started)
				<-release
			case 2:
				close(second)
			}
		})
	}()
	notifyProjectionChange(changes)
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first refresh did not begin")
	}
	for i := 0; i < 100; i++ {
		notifyProjectionChange(changes)
	}
	close(release)
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("events arriving during refresh were lost")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
	require.EqualValues(t, 2, count.Load())
}

type controlledProjectionQuery struct {
	mu     sync.Mutex
	result interface{}
	err    error
}

func (q *controlledProjectionQuery) Search(_ interface{}) (interface{}, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.result, q.err
}

func (q *controlledProjectionQuery) set(result interface{}, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.result = result
	q.err = err
}

func TestRecomputeProjectionsPreservesLastSuccessfulValue(t *testing.T) {
	good := &controlledProjectionQuery{result: "new"}
	flaky := &controlledProjectionQuery{err: assert.AnError}
	e := &entry{
		lister: &mockLister{objects: []runtime.Object{
			&unstructured.Unstructured{Object: map[string]interface{}{"kind": "ConfigMap"}},
		}},
		gce: &kyvernov2beta1.GlobalContextEntry{
			ObjectMeta: metav1.ObjectMeta{Name: "projection-test"},
		},
		eventGen: &mockEventGen{},
		projections: []store.Projection{
			{Name: "good", JP: good},
			{Name: "flaky", JP: flaky},
		},
		projected: map[string]interface{}{
			"good": "old",
			"flaky": "last-valid",
		},
	}
	e.recomputeProjections()
	got, err := e.Get("good")
	require.NoError(t, err)
	assert.Equal(t, "new", got)
	got, err = e.Get("flaky")
	require.NoError(t, err)
	assert.Equal(t, "last-valid", got)
	assert.Len(t, e.eventGen.(*mockEventGen).events, 1)

	flaky.set("recovered", nil)
	e.recomputeProjections()
	got, err = e.Get("flaky")
	require.NoError(t, err)
	assert.Equal(t, "recovered", got)
}

func TestRecomputeProjectionsKeepsSnapshotOnListerError(t *testing.T) {
	e := &entry{
		lister: &mockLister{err: assert.AnError},
		gce: &kyvernov2beta1.GlobalContextEntry{},
		eventGen: &mockEventGen{},
		projections: []store.Projection{{Name: "stable", JP: &controlledProjectionQuery{result: "changed"}}},
		projected: map[string]interface{}{"stable": "previous"},
	}
	e.recomputeProjections()
	got, err := e.Get("stable")
	require.NoError(t, err)
	assert.Equal(t, "previous", got)
	assert.Len(t, e.eventGen.(*mockEventGen).events, 1)
}
