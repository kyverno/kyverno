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
			Name: "test-cm",
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
		Namespace: "test-ns",
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

	rawObj, ok := activation.Request["object"].(map[string]any)
	require.True(t, ok)
	meta, ok := rawObj["metadata"].(map[string]any)
	require.True(t, ok)
	_, hasNs := meta["namespace"]
	assert.False(t, hasNs, "raw builder preserves object metadata without injecting request.namespace")
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

func TestPrepareK8sActivation_Delete(t *testing.T) {
	oldObj := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "test-ns"},
		Data:       map[string]string{"foo": "delete-me"},
	}
	oldRaw, err := json.Marshal(oldObj)
	require.NoError(t, err)

	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		Operation: admissionv1.Delete,
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}
	attr := admission.NewAttributesRecord(
		nil,
		oldObj,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		"test-ns",
		"test-cm",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
		"",
		admission.Delete,
		nil,
		false,
		nil,
	)

	activation, err := PrepareK8sActivation(attr, req, nil, nil, BuildRawRequestMap)
	require.NoError(t, err)
	assert.Nil(t, activation.Object)
	assert.NotNil(t, activation.OldObject)
}

func TestPrepareK8sActivation_Connect(t *testing.T) {
	req := &admissionv1.AdmissionRequest{
		UID:       "test-uid",
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Operation: admissionv1.Connect,
	}
	attr := admission.NewAttributesRecord(
		nil,
		nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		"test-ns",
		"test-pod",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		"exec",
		admission.Connect,
		nil,
		false,
		nil,
	)

	activation, err := PrepareK8sActivation(attr, req, nil, nil, BuildRawRequestMap)
	require.NoError(t, err)
	assert.Nil(t, activation.Object)
	assert.Nil(t, activation.OldObject)
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

func isInsideSyncOnceValues(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		call, ok := stack[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "OnceValues" {
			return true
		}
	}
	return false
}

// inspectActivationAssemblyErrors checks an AST file for forbidden activation assembly or
// un-memoized request-map builder calls.
func inspectActivationAssemblyErrors(path string, node *ast.File) []string {
	var errs []string
	norm := filepath.ToSlash(path)

	var stack []ast.Node
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		stack = append(stack, n)

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		name := sel.Sel.Name
		switch name {
		case "ObjectToResolveVal":
			if !strings.HasSuffix(norm, "/pkg/cel/compiler/activation.go") &&
				!strings.HasSuffix(norm, "/pkg/cel/policies/dpol/compiler/policy.go") {
				errs = append(errs, fmt.Sprintf("Forbidden hand-rolled activation assembly in %s: engines must delegate to compiler.PrepareK8sActivation", path))
			}
		case "ConvertObjectToUnstructured":
			if !strings.HasSuffix(norm, "/pkg/cel/compiler/requestmap.go") &&
				!strings.HasSuffix(norm, "/pkg/admissionpolicy/mutate.go") {
				errs = append(errs, fmt.Sprintf("Forbidden ConvertObjectToUnstructured call in %s: engines must delegate request map creation to compiler.PrepareK8sActivation", path))
			}
		case "BuildRawRequestMap", "BuildNormalizedRequestMap", "BuildNormalizedRequestMapFromResources":
			if strings.HasSuffix(norm, "/pkg/cel/compiler/requestmap.go") {
				return true
			}
			memoized := isInsideSyncOnceValues(stack)
			allowedFile := strings.HasSuffix(norm, "/pkg/cel/policies/vpol/engine/engine.go") ||
				strings.HasSuffix(norm, "/pkg/cel/policies/mpol/engine/engine.go") ||
				strings.HasSuffix(norm, "/pkg/cel/policies/ivpol/engine/engine.go") ||
				strings.HasSuffix(norm, "/pkg/image/verification/evaluator/eval.go")
			if !allowedFile || !memoized {
				errs = append(errs, fmt.Sprintf("Direct %s call in %s (memoized=%v): pass the builder to compiler.PrepareK8sActivation as fallback instead", name, path, memoized))
			}
		}
		return true
	})
	return errs
}

// TestActivationAssembly_Guard_NoHandRolledActivation scans the entire module to guarantee
// no engine or caller hand-rolls CEL admission activation assembly, extracts attr.GetObject/attr.GetOldObject
// through reflection or ConvertObjectToUnstructured, or invokes request-map builders outside sanctioned memoization.
func TestActivationAssembly_Guard_NoHandRolledActivation(t *testing.T) {
	moduleRoot := filepath.Join("..", "..", "..")

	err := filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == ".github" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		node, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		errs := inspectActivationAssemblyErrors(path, node)
		for _, errStr := range errs {
			t.Error(errStr)
		}
		return nil
	})
	require.NoError(t, err)

	t.Run("NegativeFixtures", func(t *testing.T) {
		fset := token.NewFileSet()

		// 1. Un-memoized builder call inside a policy loop in an allowlisted file
		badLoopSrc := `package evaluator
import "github.com/kyverno/kyverno/pkg/cel/compiler"
func evaluateLoop() {
	for i := 0; i < 10; i++ {
		compiler.BuildRawRequestMap(nil)
	}
}`
		badNode, err := parser.ParseFile(fset, "eval.go", badLoopSrc, 0)
		require.NoError(t, err)
		errs := inspectActivationAssemblyErrors("/pkg/image/verification/evaluator/eval.go", badNode)
		assert.NotEmpty(t, errs, "guard must reject un-memoized builder call in eval.go loop")

		// 2. ConvertObjectToUnstructured inside an engine
		badConvertSrc := `package vpol
import "github.com/kyverno/kyverno/pkg/cel/utils"
func eval() {
	utils.ConvertObjectToUnstructured(nil)
}`
		convertNode, err := parser.ParseFile(fset, "eval.go", badConvertSrc, 0)
		require.NoError(t, err)
		errs = inspectActivationAssemblyErrors("/pkg/cel/policies/vpol/compiler/eval.go", convertNode)
		assert.NotEmpty(t, errs, "guard must reject ConvertObjectToUnstructured in engine compiler")

		// 3. ObjectToResolveVal inside an unauthorized engine
		badResolveSrc := `package ivpol
import "github.com/kyverno/kyverno/pkg/cel/utils"
func eval() {
	utils.ObjectToResolveVal(nil)
}`
		resolveNode, err := parser.ParseFile(fset, "eval.go", badResolveSrc, 0)
		require.NoError(t, err)
		errs = inspectActivationAssemblyErrors("/pkg/cel/policies/ivpol/compiler/eval.go", resolveNode)
		assert.NotEmpty(t, errs, "guard must reject ObjectToResolveVal in ivpol compiler")

		// 4. BuildNormalizedRequestMapFromResources outside sync.OnceValues
		badResSrc := `package mpol
import "github.com/kyverno/kyverno/pkg/cel/compiler"
func eval() {
	compiler.BuildNormalizedRequestMapFromResources(nil, nil, nil)
}`
		resNode, err := parser.ParseFile(fset, "eval.go", badResSrc, 0)
		require.NoError(t, err)
		errs = inspectActivationAssemblyErrors("/pkg/cel/policies/mpol/compiler/eval.go", resNode)
		assert.NotEmpty(t, errs, "guard must reject BuildNormalizedRequestMapFromResources outside memoization")
	})
}
