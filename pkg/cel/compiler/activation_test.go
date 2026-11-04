package compiler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
)

func TestPrepareK8sActivation_NilAttr(t *testing.T) {
	_, err := PrepareK8sActivation(nil, nil, nil, nil, BuildRawRequestMap)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot evaluate Kubernetes-mode policy without admission attributes")
}

func TestPrepareK8sActivation_NilFallback(t *testing.T) {
	cmObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	attr := admission.NewAttributesRecord(cmObj, nil, schema.GroupVersionKind{}, "", "test-cm", schema.GroupVersionResource{}, "", admission.Create, nil, false, nil)
	_, err := PrepareK8sActivation(attr, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "request map builder must not be nil")
}

func TestPrepareK8sActivation_RawFallback(t *testing.T) {
	cmObj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-ns",
		},
		Data: map[string]string{"foo": "bar"},
	}
	cmRaw, err := json.Marshal(cmObj)
	require.NoError(t, err)

	nsObj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ns",
		},
	}

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: cmRaw},
	}

	attr := admission.NewAttributesRecord(
		cmObj,
		nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"test-ns",
		"test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)

	activation, err := PrepareK8sActivation(attr, req, nsObj, nil, BuildRawRequestMap)
	require.NoError(t, err)
	require.NotNil(t, activation.Object)
	require.Nil(t, activation.OldObject)
	require.NotNil(t, activation.Namespace)
	require.NotNil(t, activation.Request)

	m := activation.ToMap()
	assert.Equal(t, activation.Object, m[ObjectKey])
	assert.Equal(t, activation.OldObject, m[OldObjectKey])
	assert.Equal(t, activation.Namespace, m[NamespaceObjectKey])
	assert.Equal(t, activation.Request, m[RequestKey])
}

func TestPrepareK8sActivation_Update(t *testing.T) {
	oldObj := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
		Data:       map[string]string{"foo": "old"},
	}
	newObj := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
		Data:       map[string]string{"foo": "new"},
	}
	oldRaw, err := json.Marshal(oldObj)
	require.NoError(t, err)
	newRaw, err := json.Marshal(newObj)
	require.NoError(t, err)

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		Operation: admissionv1.Update,
		Object:    runtime.RawExtension{Raw: newRaw},
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}
	attr := admission.NewAttributesRecord(
		newObj,
		oldObj,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"test-ns",
		"test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"",
		admission.Update,
		nil,
		false,
		nil,
	)

	activation, err := PrepareK8sActivation(attr, req, nil, nil, BuildRawRequestMap)
	require.NoError(t, err)
	require.NotNil(t, activation.Object)
	require.NotNil(t, activation.OldObject)
	assert.NotEqual(t, activation.Object, activation.OldObject)
}

func TestPrepareK8sActivation_NormalizedFallback(t *testing.T) {
	cmObj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-ns",
		},
	}
	cmRaw, err := json.Marshal(cmObj)
	require.NoError(t, err)

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: cmRaw},
	}

	attr := admission.NewAttributesRecord(
		cmObj,
		nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"test-ns",
		"test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)

	activation, err := PrepareK8sActivation(attr, req, nil, nil, BuildNormalizedRequestMap)
	require.NoError(t, err)
	require.NotNil(t, activation.Request)

	// Normalized builder populates oldObject key with typed-nil map on CREATE
	oldObj, exists := activation.Request["oldObject"]
	assert.True(t, exists, "the oldObject key must be present even when empty")
	assert.Nil(t, oldObj)
}

func TestPrepareK8sActivation_MemoizedRequestMapFn(t *testing.T) {
	cmObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cm",
		},
	}
	attr := admission.NewAttributesRecord(
		cmObj,
		nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"",
		"test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"",
		admission.Create,
		nil,
		false,
		nil,
	)

	memoMap := map[string]any{"memoized": true}
	calledFn := false
	reqFn := func() (map[string]any, error) {
		calledFn = true
		return memoMap, nil
	}

	fallbackCalled := false
	fallback := func(*admissionv1.AdmissionRequest) (map[string]any, error) {
		fallbackCalled = true
		return nil, nil
	}

	activation, err := PrepareK8sActivation(attr, nil, nil, reqFn, fallback)
	require.NoError(t, err)
	assert.True(t, calledFn)
	assert.False(t, fallbackCalled)
	assert.Equal(t, memoMap, activation.Request)
}

func TestPrepareK8sActivation_MemoizedRequestMapFnError(t *testing.T) {
	cmObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	attr := admission.NewAttributesRecord(cmObj, nil, schema.GroupVersionKind{}, "", "test-cm", schema.GroupVersionResource{}, "", admission.Create, nil, false, nil)
	errBuild := fmt.Errorf("request map build failed")
	reqFn := func() (map[string]any, error) {
		return nil, errBuild
	}
	fallbackCalled := false
	fallback := func(*admissionv1.AdmissionRequest) (map[string]any, error) {
		fallbackCalled = true
		return nil, nil
	}
	_, err := PrepareK8sActivation(attr, nil, nil, reqFn, fallback)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to prepare request variable for evaluation")
	assert.False(t, fallbackCalled)
}

// Engine call sites allowed to invoke a request-map builder directly to construct the memoized thunk.
var allowedRequestMapCalls = map[string]string{
	"/vpol/engine/engine.go":                "BuildRawRequestMap",
	"/ivpol/engine/engine.go":               "BuildRawRequestMap",
	"/image/verification/evaluator/eval.go": "BuildRawRequestMap",
	"/mpol/engine/engine.go":                "BuildNormalizedRequestMap",
}

func isAllowedRequestMapCall(path, name string) bool {
	norm := filepath.ToSlash(path)
	for suffix, builder := range allowedRequestMapCalls {
		if strings.HasSuffix(norm, suffix) && builder == name {
			return true
		}
	}
	return false
}

// TestActivationAssembly_Guard_NoHandRolledActivation scans the engine policy packages
// to guarantee no caller hand-rolls CEL admission activation assembly or directly extracts
// attr.GetObject/attr.GetOldObject through reflection utils instead of using PrepareK8sActivation.
func TestActivationAssembly_Guard_NoHandRolledActivation(t *testing.T) {
	rootDirs := []string{
		filepath.Join("..", "policies"),
		filepath.Join("..", "..", "image", "verification"),
	}

	for _, root := range rootDirs {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			// dpol operates exclusively on pre-materialized unstructured objects without admission
			// attributes or requests, and is intentionally excluded per design.
			if strings.Contains(path, filepath.Join("policies", "dpol")) {
				return nil
			}

			fset := token.NewFileSet()
			node, parseErr := parser.ParseFile(fset, path, nil, 0)
			if parseErr != nil {
				return parseErr
			}

			ast.Inspect(node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				// Check for direct calls to utils.ObjectToResolveVal
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == "ObjectToResolveVal" {
					t.Errorf("Forbidden hand-rolled activation assembly in %s: engines must delegate to compiler.PrepareK8sActivation", path)
				}
				if name := sel.Sel.Name; name == "BuildRawRequestMap" || name == "BuildNormalizedRequestMap" {
					if !isAllowedRequestMapCall(path, name) {
						t.Errorf("Direct %s call in %s: pass the builder to compiler.PrepareK8sActivation as the fallback instead", name, path)
					}
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
}
