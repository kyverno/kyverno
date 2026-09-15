package compiler

import (
	"reflect"
	"testing"

	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func buildTestRequestAndAttr(t *testing.T) (*admissionv1.AdmissionRequest, admission.Attributes, *unstructured.Unstructured, *unstructured.Unstructured) {
	t.Helper()
	objRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx:1.21"}]}}`)
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "nginx", "namespace": "default"},
		"spec": map[string]any{
			"containers": []any{map[string]any{"name": "app", "image": "nginx:1.21"}},
		},
	}}
	oldObject := &unstructured.Unstructured{}

	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: objRaw},
	}

	attr := admission.NewAttributesRecord(
		object,
		oldObject,
		schema.GroupVersionKind(request.Kind),
		request.Namespace,
		request.Name,
		schema.GroupVersionResource(request.Resource),
		request.SubResource,
		admission.Operation(request.Operation),
		nil,
		false,
		nil,
	)
	return request, attr, object, oldObject
}

// TestPrepareK8sData_HoistedVsNilRequestMap asserts that a caller-supplied
// (hoisted) requestMap and the nil-fallback path (which builds its own via
// compiler.BuildRawRequestMap) produce identical Request content, so
// threading the hoisted map through prepareK8sData is behavior-preserving.
func TestPrepareK8sData_HoistedVsNilRequestMap(t *testing.T) {
	request, attr, _, _ := buildTestRequestAndAttr(t)

	hoisted, err := celcompiler.BuildRawRequestMap(request)
	require.NoError(t, err)
	require.NotNil(t, hoisted)

	withHoist, err := prepareK8sData(attr, request, nil, func() (map[string]any, error) { return hoisted, nil }, nil)
	require.NoError(t, err)

	withNil, err := prepareK8sData(attr, request, nil, nil, nil)
	require.NoError(t, err)

	assert.True(t, reflect.DeepEqual(withHoist.Request, withNil.Request),
		"hoisted requestMap and nil-fallback-built requestMap must produce identical Request content")
	assert.Equal(t, hoisted, withHoist.Request, "prepareK8sData must use the hoisted map unchanged, not rebuild it")

	// Object/OldObject/Namespace are unaffected by the requestMap plumbing.
	assert.Equal(t, withHoist.Object, withNil.Object)
	assert.Equal(t, withHoist.OldObject, withNil.OldObject)
}

// TestPrepareK8sData_NoAttributes preserves the existing guard: Kubernetes-
// mode evaluation without admission.Attributes must error clearly.
func TestPrepareK8sData_NoAttributes(t *testing.T) {
	_, err := prepareK8sData(nil, nil, nil, nil, nil)
	assert.Error(t, err)
}

// buildLargeTestRequestAndAttr is like buildTestRequestAndAttr but with a
// ~50KB admitted object, so TestPrepareK8sData_AllocsCeiling actually proves
// the allocation ceiling is independent of object size, not just low for a
// tiny fixture.
func buildLargeTestRequestAndAttr(t *testing.T) (*admissionv1.AdmissionRequest, admission.Attributes, *unstructured.Unstructured, *unstructured.Unstructured) {
	t.Helper()
	padding := make([]byte, 50*1024)
	for i := range padding {
		padding[i] = byte('a' + i%26)
	}
	containers := make([]any, 0, 20)
	for i := 0; i < 20; i++ {
		containers = append(containers, map[string]any{
			"name":  "app",
			"image": "nginx:1.21",
			"env":   []any{map[string]any{"name": "PADDING", "value": string(padding[:2500])}},
		})
	}
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": "nginx", "namespace": "default"},
		"spec":       map[string]any{"containers": containers},
	}}
	objRaw, err := object.MarshalJSON()
	require.NoError(t, err)
	oldObject := &unstructured.Unstructured{}

	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: objRaw},
	}

	attr := admission.NewAttributesRecord(
		object,
		oldObject,
		schema.GroupVersionKind(request.Kind),
		request.Namespace,
		request.Name,
		schema.GroupVersionResource(request.Resource),
		request.SubResource,
		admission.Operation(request.Operation),
		nil,
		false,
		nil,
	)
	return request, attr, object, oldObject
}

// TestPrepareK8sData_AllocsCeiling is a CI-assertable stand-in for the
// engine-level BenchmarkHandle: with a pre-built (hoisted) requestMap,
// prepareK8sData must no longer re-run the expensive
// ConvertObjectToUnstructured(request) reflective walk per call, so its
// per-call allocation count should be small and constant even for a
// realistically large (~50KB) admitted object.
func TestPrepareK8sData_AllocsCeiling(t *testing.T) {
	request, attr, _, _ := buildLargeTestRequestAndAttr(t)
	hoisted, err := celcompiler.BuildRawRequestMap(request)
	require.NoError(t, err)
	requestMapFn := func() (map[string]any, error) { return hoisted, nil }

	const allocCeiling = 50
	allocs := testing.AllocsPerRun(100, func() {
		if _, err := prepareK8sData(attr, request, nil, requestMapFn, nil); err != nil {
			t.Fatalf("prepareK8sData failed: %v", err)
		}
	})
	assert.LessOrEqual(t, allocs, float64(allocCeiling),
		"prepareK8sData with a pre-built requestMap should stay under a small constant allocation ceiling, independent of admitted object size")
}

// TestPrepareK8sData_RequestMapFnInvokedOnlyWhenReached proves prepareK8sData
// calls requestMapFn - it doesn't skip it and doesn't call it more than once
// per invocation - so the memoization contract sync.OnceValues callers rely
// on (see vpol/engine/engine.go) is actually exercised through this call
// site.
func TestPrepareK8sData_RequestMapFnInvokedOnlyWhenReached(t *testing.T) {
	request, attr, _, _ := buildTestRequestAndAttr(t)
	calls := 0
	requestMapFn := func() (map[string]any, error) {
		calls++
		return celcompiler.BuildRawRequestMap(request)
	}

	_, err := prepareK8sData(attr, request, nil, requestMapFn, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "prepareK8sData must call requestMapFn exactly once per invocation")
}
