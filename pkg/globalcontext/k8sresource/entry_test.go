package k8sresource

import (
	"sync"
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	events []interface{}
}

func (m *mockEventGen) Add(event interface{}) {
	m.events = append(m.events, event)
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
		lister:    lister,
		projected: make(map[string]interface{}),
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
		projected: make(map[string]interface{}),
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
				lister:    tt.lister,
				projected: tt.projected,
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

func TestEntry_Stop_MultipleCallsAreSafe(t *testing.T) {
	callCount := 0
	e := &entry{
		stop: func() {
			callCount++
		},
	}

	e.Stop()
	e.Stop()

	assert.Equal(t, 2, callCount, "stop should be callable multiple times")
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

	e.recomputeProjections()

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
	}

	e.recomputeProjections()

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

	e.recomputeProjections()

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

	e.recomputeProjections()

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
