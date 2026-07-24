package k8sresource

import (
	"sync"
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/event"
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
	objects   []runtime.Object
	getObject runtime.Object
	err       error
	getErr    error
}

type mockNamespaceLister struct {
	getObject runtime.Object
	err       error
}

func (m *mockNamespaceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	return nil, nil
}

func (m *mockNamespaceLister) Get(name string) (runtime.Object, error) {
	return m.getObject, m.err
}

func (m *mockLister) List(selector labels.Selector) ([]runtime.Object, error) {
	return m.objects, m.err
}

func (m *mockLister) Get(name string) (runtime.Object, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getObject, m.err
}

func (m *mockLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &mockNamespaceLister{
		getObject: m.getObject,
		err:       m.err,
	}
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

type capturingQuery struct {
	fn func(interface{}) (interface{}, error)
}

func (c *capturingQuery) Search(data interface{}) (interface{}, error) {
	return c.fn(data)
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
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{},
			},
		},
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
		gce         *kyvernov2beta1.GlobalContextEntry
	}{
		{
			name:       "empty projection returns list",
			projection: "",
			lister: &mockLister{
				objects: []runtime.Object{
					&unstructured.Unstructured{Object: map[string]interface{}{"name": "obj1"}},
				},
			},

			projected: make(map[string]interface{}),
			gce: &kyvernov2beta1.GlobalContextEntry{
				Spec: kyvernov2beta1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2beta1.KubernetesResource{},
				},
			},

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
			projected: make(map[string]interface{}),
			gce: &kyvernov2beta1.GlobalContextEntry{
				Spec: kyvernov2beta1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2beta1.KubernetesResource{},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &entry{
				lister:      tt.lister,
				gce:         tt.gce,
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
		projectedMu: sync.RWMutex{},
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

func TestEntry_Get_WithName_Namespaced(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "service-registry",
				"namespace": "kyverno",
			},
		},
	}

	lister := &mockLister{getObject: obj}

	e := &entry{
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{
					Namespace: "kyverno",
					Name:      "service-registry",
				},
			},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	result, err := e.Get("")
	assert.NoError(t, err)
	objMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	metadata := objMap["metadata"].(map[string]interface{})
	assert.Equal(t, "service-registry", metadata["name"])
}

func TestEntry_Get_WithName_ClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "my-node",
			},
		},
	}

	lister := &mockLister{getObject: obj}

	e := &entry{
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{
					Name: "my-node",
				},
			},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	result, err := e.Get("")
	assert.NoError(t, err)
	objMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	metadata := objMap["metadata"].(map[string]interface{})
	assert.Equal(t, "my-node", metadata["name"])
}

func TestEntry_Get_WithoutName(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "service-registry",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
	}

	e := &entry{
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{
					Name: "",
				},
			},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	result, err := e.Get("")
	assert.NoError(t, err)
	list, ok := result.([]interface{})
	assert.True(t, ok)
	assert.Len(t, list, 1)
}

func TestEntry_RecomputeProjections_WithName(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "service-registry",
				"namespace": "kyverno",
			},
			"data": map[string]interface{}{
				"allowedRegistries": "myregistry.io",
			},
		},
	}

	lister := &mockLister{getObject: obj}
	eventGen := &mockEventGen{}

	var receivedData interface{}
	captureQuery := &capturingQuery{fn: func(data interface{}) (interface{}, error) {
		receivedData = data
		return "myregistry.io", nil
	}}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			KubernetesResource: &kyvernov2beta1.KubernetesResource{
				Namespace: "kyverno",
				Name:      "service-registry",
			},
		},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "registry-proj", JP: captureQuery},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections()

	assert.Equal(t, "myregistry.io", e.projected["registry-proj"])
	assert.Empty(t, eventGen.events)

	// Key assertion: JMESPath must receive a single object map, not a list
	_, isList := receivedData.([]interface{})
	assert.False(t, isList, "projection should receive single object, not a list")
	_, isMap := receivedData.(map[string]interface{})
	assert.True(t, isMap, "projection should receive object as map[string]interface{}")
}

func TestEntry_GetObject_CrossNamespace_SingleMatch(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "team-a",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj},
		getErr:  assert.AnError,
	}

	e := &entry{
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{
					Name: "my-config",
				},
			},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	result, err := e.getObject("", "my-config")
	assert.NoError(t, err)
	objMap, ok := result.(map[string]interface{})
	assert.True(t, ok)
	metadata := objMap["metadata"].(map[string]interface{})
	assert.Equal(t, "my-config", metadata["name"])
}

func TestEntry_GetObject_CrossNamespace_DuplicateNameError(t *testing.T) {
	obj1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "team-a",
			},
		},
	}
	obj2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "team-b",
			},
		},
	}

	lister := &mockLister{
		objects: []runtime.Object{obj1, obj2},
		getErr:  assert.AnError,
	}

	e := &entry{
		lister: lister,
		gce: &kyvernov2beta1.GlobalContextEntry{
			Spec: kyvernov2beta1.GlobalContextEntrySpec{
				KubernetesResource: &kyvernov2beta1.KubernetesResource{
					Name: "my-config",
				},
			},
		},
		projected:   make(map[string]interface{}),
		projectedMu: sync.RWMutex{},
	}

	_, err := e.getObject("", "my-config")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple objects named")
	assert.Contains(t, err.Error(), "disambiguate")
}

func TestEntry_RecomputeProjections_WithName_ClearsOnError(t *testing.T) {
	lister := &mockLister{
		getErr: assert.AnError,
		err: assert.AnError,
	}
	eventGen := &mockEventGen{}

	gce := &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			KubernetesResource: &kyvernov2beta1.KubernetesResource{
				Namespace: "kyverno",
				Name:      "service-registry",
			},
		},
	}

	e := &entry{
		lister:   lister,
		eventGen: eventGen,
		gce:      gce,
		projections: []store.Projection{
			{Name: "registry-proj", JP: &mockJMESPathQuery{result: "stale"}},
		},
		projected: map[string]interface{}{
			"registry-proj": "stale-value",
		},
		projectedMu: sync.RWMutex{},
	}

	e.recomputeProjections()

	// Projections must be cleared when named object lookup fails
	assert.Empty(t, e.projected, "stale projections should be cleared on lookup error")
	assert.Len(t, eventGen.events, 1, "error event should be generated")
}
