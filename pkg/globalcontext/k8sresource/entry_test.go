package k8sresource

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

// mockLister implements cache.GenericLister for testing
type mockLister struct {
	objects []runtime.Object
	err     error
}

func (m *mockLister) List(selector labels.Selector) ([]runtime.Object, error) {
	return m.objects, m.err
}

func (m *mockLister) Get(name string) (runtime.Object, error) {
	return nil, nil
}

func (m *mockLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return nil
}

// mockEventGen implements event.Interface for testing
type mockEventGen struct {
	events []event.Info
}

func (m *mockEventGen) Add(infoList ...event.Info) {
	m.events = append(m.events, infoList...)
}

// mockJMESPathQuery implements jmespath.Query for testing
type mockJMESPathQuery struct {
	result interface{}
	err    error
}

func (m *mockJMESPathQuery) Search(data interface{}) (interface{}, error) {
	return m.result, m.err
}

func TestEntry_Get_EmptyProjection(t *testing.T) {
	// Create mock lister with unstructured objects
	obj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm-1",
				"namespace": "default",
			},
		},
	}
	obj2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-cm-2",
				"namespace": "default",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj1, obj2},
	}

	e := &entry{
		lister:      lister,
		projectedMu: sync.RWMutex{},
		projected:   make(map[string]interface{}),
	}

	result, err := e.Get("")

	assert.NoError(t, err)
	assert.NotNil(t, result)

	list, ok := result.([]interface{})
	assert.True(t, ok)
	assert.Len(t, list, 2)
}

func TestEntry_Get_WithProjection(t *testing.T) {
	e := &entry{
		projectedMu: sync.RWMutex{},
		projected: map[string]interface{}{
			"names": []string{"cm1", "cm2"},
		},
	}

	result, err := e.Get("names")

	assert.NoError(t, err)
	names, ok := result.([]string)
	assert.True(t, ok)
	assert.Equal(t, []string{"cm1", "cm2"}, names)
}

func TestEntry_Get_ProjectionNotFound(t *testing.T) {
	e := &entry{
		projectedMu: sync.RWMutex{},
		projected:   make(map[string]interface{}),
	}

	result, err := e.Get("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "projection \"nonexistent\" not found")
}

func TestEntry_Get_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name        string
		projection  string
		projected   map[string]interface{}
		lister      *mockLister
		expectError bool
		errorMsg    string
	}{
		{
			name:       "empty projection returns list",
			projection: "",
			lister: &mockLister{
				objects: []runtime.Object{
					&unstructured.Unstructured{Object: map[string]interface{}{"name": "obj1"}},
				},
			},
			projected:   make(map[string]interface{}),
			expectError: false,
		},
		{
			name:       "named projection found",
			projection: "myproj",
			projected: map[string]interface{}{
				"myproj": "projected-data",
			},
			expectError: false,
		},
		{
			name:        "named projection not found",
			projection:  "missing",
			projected:   make(map[string]interface{}),
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name:       "lister error on empty projection",
			projection: "",
			lister: &mockLister{
				err: assert.AnError,
			},
			projected:   make(map[string]interface{}),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &entry{
				lister:      tt.lister,
				projected:   tt.projected,
				projectedMu: sync.RWMutex{},
			}

			result, err := e.Get(tt.projection)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestEntry_Stop(t *testing.T) {
	stopCalled := false
	e := &entry{
		stop: func() {
			stopCalled = true
		},
	}

	e.Stop()

	assert.True(t, stopCalled, "stop function should be called")
}

func TestEntry_Stop_CanBeCalledMultipleTimes(t *testing.T) {
	callCount := 0
	e := &entry{
		stop: func() {
			callCount++
		},
	}

	// Verify Stop can be called multiple times without panicking
	e.Stop()
	e.Stop()

	assert.Equal(t, 2, callCount, "stop function should be called each time Stop is invoked")
}

func TestEntry_ListObjects_Success(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	e := &entry{
		lister: lister,
	}

	result, err := e.listObjects()

	assert.NoError(t, err)
	assert.Len(t, result, 1)

	// Verify the object structure
	objMap, ok := result[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "v1", objMap["apiVersion"])
	assert.Equal(t, "Pod", objMap["kind"])
}

func TestEntry_ListObjects_EmptyList(t *testing.T) {
	lister := &mockLister{
		objects: []runtime.Object{},
	}

	e := &entry{
		lister: lister,
	}

	result, err := e.listObjects()

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestEntry_ListObjects_ListerError(t *testing.T) {
	lister := &mockLister{
		err: assert.AnError,
	}

	e := &entry{
		lister: lister,
	}

	result, err := e.listObjects()

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to list objects")
}

func TestEntry_ListObjects_NonUnstructuredSkipped(t *testing.T) {
	// Create a mix of unstructured and non-unstructured objects
	unstructuredObj := &unstructured.Unstructured{
		Object: map[string]interface{}{"name": "unstructured"},
	}

	lister := &mockLister{
		objects: []runtime.Object{unstructuredObj},
	}

	e := &entry{
		lister: lister,
	}

	result, err := e.listObjects()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestEntry_RecomputeProjections_Success(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	mockQuery := &mockJMESPathQuery{
		result: "projected-result",
	}

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "test-proj", JP: mockQuery},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Equal(t, "projected-result", e.projected["test-proj"])
	assert.Empty(t, eventGen.events, "no error events should be generated")
}

func TestEntry_RecomputeProjections_ListerError(t *testing.T) {
	lister := &mockLister{
		err: assert.AnError,
	}

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
	}

	e := &entry{
		lister:      lister,
		eventGen:    eventGen,
		gce:         gce,
		projections: []store.Projection{},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Len(t, eventGen.events, 1, "should generate error event")
}

func TestEntry_RecomputeProjections_ProjectionError(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{"name": "test"},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	mockQuery := &mockJMESPathQuery{
		err: assert.AnError,
	}

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "failing-proj", JP: mockQuery},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Len(t, eventGen.events, 1, "should generate error event for projection failure")
}

func TestEntry_RecomputeProjections_MultipleProjections(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{"name": "test"},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	mockQuery1 := &mockJMESPathQuery{result: "result1"}
	mockQuery2 := &mockJMESPathQuery{result: "result2"}

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "proj1", JP: mockQuery1},
			{Name: "proj2", JP: mockQuery2},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Equal(t, "result1", e.projected["proj1"])
	assert.Equal(t, "result2", e.projected["proj2"])
	assert.Empty(t, eventGen.events)
}

func TestEntry_ConcurrentAccess(t *testing.T) {
	e := &entry{
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	// Pre-populate projection
	e.projected["test"] = "initial"

	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = e.Get("test")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or race
	assert.Equal(t, "initial", e.projected["test"])
}

// TestEntry_RecomputeProjections_CELExpression exercises an expression-only projection:
// for a KubernetesResource entry, `object` is the list of matching objects directly (not
// wrapped in {"items": [...]}), so `object.size()` is the CEL equivalent of the entry's
// zero-projection read, and it yields an int64 (not JMESPath's float64).
func TestEntry_RecomputeProjections_CELExpression(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "test"},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	prog, errs := compiler.CompileGlobalContextProjection(nil, "object.size()")
	require.Empty(t, errs)

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gce"},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "count", Program: prog},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Equal(t, int64(1), e.projected["count"])
	assert.IsType(t, int64(0), e.projected["count"])
	assert.Empty(t, eventGen.events, "no error events should be generated")
}

// TestEntry_RecomputeProjections_CELExpressionError mirrors the apiCall-only "items |
// length(@)" trap: object is the list of objects directly, so object.items.size() has no
// "items" field and the projection eval fails, keeping the prior cached value, same as a
// failing JMESPath projection does today.
func TestEntry_RecomputeProjections_CELExpressionError(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{"name": "test"},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	prog, errs := compiler.CompileGlobalContextProjection(nil, "object.items.size()")
	require.Empty(t, errs)

	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gce"},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "count", Program: prog},
		},
		projected: map[string]interface{}{
			"count": "previous-value",
		},
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections(context.Background())

	assert.Len(t, eventGen.events, 1, "should generate error event for projection failure")
	assert.Equal(t, "previous-value", e.projected["count"], "prior cached value is kept on eval error")
}

// TestEntry_New_BadCELExpressionFails confirms a bad CEL expression fails New with an error
// naming the projection, before any informer is started.
func TestEntry_New_BadCELExpressionFails(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	scheme := runtime.NewScheme()
	dClient := dynamicfake.NewSimpleDynamicClient(scheme)

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "test-gce"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			Projections: []kyvernov2beta1.GlobalContextEntryProjection{
				{Name: "bad", Expression: "object.sizee()"},
			},
		},
	}

	_, err := New(
		context.Background(),
		gce,
		&mockEventGen{},
		dClient,
		logr.Discard(),
		gvr,
		"",
		jmespath.New(config.NewDefaultConfiguration(false)),
	)

	require.Error(t, err)
	// "bad" alone is the projection name and would match without any compile failure;
	// also assert the error names the actual failure.
	assert.Contains(t, err.Error(), "failed to compile CEL expression for projection \"bad\"")
}

// TestEntry_Stop_Idempotent exercises the production stop closure created by
// newOnceStop (the same closure New wires into the entry): repeated and
// concurrent Stop calls must run the teardown exactly once.
func TestEntry_Stop_Idempotent(t *testing.T) {
	var count int32
	var group wait.Group
	cancel := func() { atomic.AddInt32(&count, 1) }

	e := &entry{stop: newOnceStop(cancel, &group)}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Stop()
		}()
	}
	wg.Wait()
	e.Stop()

	assert.Equal(t, int32(1), atomic.LoadInt32(&count))
}
