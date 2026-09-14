package compiler

import (
	"reflect"
	"testing"

	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// TestPrepareData_HoistedVsNilRequestMap asserts that a caller-supplied
// (hoisted) requestMap and the nil-fallback path (which builds its own via
// compiler.BuildNormalizedRequestMap) produce identical `request` activation
// content, so threading the hoisted map through prepareData is
// behavior-preserving.
func TestPrepareData_HoistedVsNilRequestMap(t *testing.T) {
	objRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx:1.21"}]}}`)
	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: objRaw},
	}

	attr := &mockAttributes{
		obj:    &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "nginx"}}},
		oldObj: nil,
	}

	hoisted, err := celcompiler.BuildNormalizedRequestMap(request)
	require.NoError(t, err)
	require.NotNil(t, hoisted)

	withHoist, err := prepareData(attr, request, &corev1.Namespace{}, func() (map[string]any, error) { return hoisted, nil })
	require.NoError(t, err)

	withNil, err := prepareData(attr, request, &corev1.Namespace{}, nil)
	require.NoError(t, err)

	assert.True(t, reflect.DeepEqual(withHoist[celcompiler.RequestKey], withNil[celcompiler.RequestKey]),
		"hoisted requestMap and nil-fallback-built requestMap must produce identical `request` content")
	assert.Equal(t, hoisted, withHoist[celcompiler.RequestKey], "prepareData must use the hoisted map unchanged, not rebuild it")
}

// TestPrepareData_NoAttributes preserves the existing guard: Kubernetes-mode
// evaluation without admission.Attributes must error clearly.
func TestPrepareData_NoAttributes(t *testing.T) {
	_, err := prepareData(nil, nil, nil, nil)
	assert.Error(t, err)
}

// TestPrepareData_RequestMapFnInvokedOnlyWhenReached proves prepareData
// calls requestMapFn - it doesn't skip it and doesn't call it more than once
// per invocation - so the memoization contract sync.OnceValues callers rely
// on (see mpol/engine/engine.go) is actually exercised through this call
// site.
func TestPrepareData_RequestMapFnInvokedOnlyWhenReached(t *testing.T) {
	objRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"default"},"spec":{"containers":[{"name":"app","image":"nginx:1.21"}]}}`)
	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
		UserInfo:  authenticationv1.UserInfo{Username: "alice"},
		Object:    runtime.RawExtension{Raw: objRaw},
	}
	attr := &mockAttributes{
		obj:    &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "nginx"}}},
		oldObj: nil,
	}

	calls := 0
	requestMapFn := func() (map[string]any, error) {
		calls++
		return celcompiler.BuildNormalizedRequestMap(request)
	}

	_, err := prepareData(attr, request, &corev1.Namespace{}, requestMapFn)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "prepareData must call requestMapFn exactly once per invocation")
}
