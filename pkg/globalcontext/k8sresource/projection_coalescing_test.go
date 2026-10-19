package k8sresource

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamic "k8s.io/client-go/dynamic/fake"
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
			"good":  "old",
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
		lister:      &mockLister{err: assert.AnError},
		gce:         &kyvernov2beta1.GlobalContextEntry{},
		eventGen:    &mockEventGen{},
		projections: []store.Projection{{Name: "stable", JP: &controlledProjectionQuery{result: "changed"}}},
		projected:   map[string]interface{}{"stable": "previous"},
	}
	e.recomputeProjections()
	got, err := e.Get("stable")
	require.NoError(t, err)
	assert.Equal(t, "previous", got)
	assert.Len(t, e.eventGen.(*mockEventGen).events, 1)
}

// startupProjectionQuery blocks the first evaluation after the informer cache
// has synchronized. Subsequent evaluations use the current lister snapshot.
type startupProjectionQuery struct {
	calls           atomic.Int32
	initialStarted  chan struct{}
	initialVersion  chan string
	continueInitial chan struct{}
	unblock         sync.Once
}

func (q *startupProjectionQuery) Search(data interface{}) (interface{}, error) {
	objects := data.([]interface{})
	var version string
	if len(objects) > 0 {
		resource := objects[0].(map[string]interface{})
		data, _ := resource["data"].(map[string]interface{})
		version, _ = data["version"].(string)
	}
	if q.calls.Add(1) == 1 {
		q.initialVersion <- version
		close(q.initialStarted)
		<-q.continueInitial
	}
	return version, nil
}

func (q *startupProjectionQuery) release() {
	q.unblock.Do(func() { close(q.continueInitial) })
}

type startupJMESPath struct {
	query jmespath.Query
}

func (s startupJMESPath) Query(string) (jmespath.Query, error) {
	return s.query, nil
}

func (s startupJMESPath) Search(_ string, data interface{}) (interface{}, error) {
	return s.query.Search(data)
}

// Regression: an update delivered while New is computing its initial snapshot
// must be observed by the worker after the bootstrap-to-worker handoff.
func TestNewRetainsUpdatesDuringInitialProjection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	query := &startupProjectionQuery{
		initialStarted:  make(chan struct{}),
		initialVersion:  make(chan string, 1),
		continueInitial: make(chan struct{}),
	}
	defer query.release()

	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	resource := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "projection-bootstrap",
			"namespace": "default",
		},
		"data": map[string]interface{}{"version": "before"},
	}}
	client := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "ConfigMapList"},
		resource,
	)
	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "projection-bootstrap"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "version", JMESPath: "[].data.version"},
			},
		},
	}
	type entryResult struct {
		entry store.Entry
		err   error
	}
	resultCh := make(chan entryResult, 1)
	changes := make(chan struct{}, 1)
	go func() {
		got, err := newWithChanges(ctx, gce, &mockEventGen{}, client, logr.Discard(), gvr, "default", startupJMESPath{query: query}, changes)
		resultCh <- entryResult{entry: got, err: err}
	}()

	select {
	case <-query.initialStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("initial projection did not start")
	}

	updated := resource.DeepCopy()
	updated.Object["data"] = map[string]interface{}{"version": "after"}
	_, err := client.Resource(gvr).Namespace("default").Update(ctx, updated, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Update only modifies the fake tracker's state synchronously. Observe
	// the informer callback notification before releasing the first query.
	require.Eventually(t, func() bool {
		return len(changes) == 1
	}, 10*time.Second, 10*time.Millisecond, "updated informer event was not delivered during startup")
	require.Equal(t, "before", <-query.initialVersion)
	query.release()

	var res entryResult
	select {
	case res = <-resultCh:
	case <-time.After(10 * time.Second):
		t.Fatal("New did not return after initial projection")
	}
	require.NoError(t, res.err)
	defer res.entry.Stop()

	require.Eventually(t, func() bool {
		value, err := res.entry.Get("version")
		return err == nil && value == "after"
	}, 10*time.Second, 10*time.Millisecond, "update during startup must reach the projection worker")
	require.GreaterOrEqual(t, query.calls.Load(), int32(2))
}
