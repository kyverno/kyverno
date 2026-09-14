package compiler

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/extensions/cel/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// podRaw returns realistic Pod JSON bytes for the given name/image, chosen to
// exercise nested maps, arrays, integers and booleans - the shapes most
// likely to reveal numeric-type drift (int64 vs float64) between the old and
// new request-map construction paths.
func podRaw(name, image string) []byte {
	return []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "` + name + `", "namespace": "default", "generation": 3},
		"spec": {
			"containers": [
				{"name": "app", "image": "` + image + `", "ports": [{"containerPort": 8080}]}
			],
			"terminationGracePeriodSeconds": 30,
			"hostNetwork": false,
			"priority": -1
		},
		"status": {"phase": "Running", "conditions": []}
	}`)
}

// rawVariantOracle is the literal base-2360fc7 formula for vpol/gpol:
// utils.ConvertObjectToUnstructured(request), verbatim, with no
// post-processing - vpol/gpol never spliced object/oldObject at base.
func rawVariantOracle(t *testing.T, request *admissionv1.AdmissionRequest) map[string]any {
	t.Helper()
	result, err := utils.ConvertObjectToUnstructured(request)
	require.NoError(t, err)
	if result == nil || result.Object == nil {
		return map[string]any{}
	}
	return result.Object
}

// normalizedVariantOracle is the literal base-2360fc7 formula for mpol:
// utils.ConvertObjectToUnstructured(request) followed by an unconditional
// splice of admissionutils.ExtractResources(nil, *request) into
// object/oldObject - exactly mpol/compiler/eval.go's base code
// (requestVal["object"] = object.Object; requestVal["oldObject"] = oldObject.Object,
// with no length guard).
func normalizedVariantOracle(t *testing.T, request *admissionv1.AdmissionRequest) map[string]any {
	t.Helper()
	result, err := utils.ConvertObjectToUnstructured(request)
	require.NoError(t, err)
	oracle := result.Object
	if oracle == nil {
		oracle = map[string]any{}
	}
	object, oldObject, err := admissionutils.ExtractResources(nil, *request)
	require.NoError(t, err)
	oracle["object"] = object.Object
	oracle["oldObject"] = oldObject.Object
	return oracle
}

// baseRequest returns a fully populated AdmissionRequest covering every
// optional field the CEL `request` contract exposes, with the given
// operation and raw object/oldObject payloads.
func baseRequest(operation admissionv1.Operation, objRaw, oldObjRaw []byte) *admissionv1.AdmissionRequest {
	dryRun := true
	req := &admissionv1.AdmissionRequest{
		UID: types.UID("11111111-2222-3333-4444-555555555555"),
		Kind: metav1.GroupVersionKind{
			Group: "", Version: "v1", Kind: "Pod",
		},
		Resource: metav1.GroupVersionResource{
			Group: "", Version: "v1", Resource: "pods",
		},
		SubResource: "status",
		RequestKind: &metav1.GroupVersionKind{
			Group: "", Version: "v1", Kind: "Pod",
		},
		RequestResource: &metav1.GroupVersionResource{
			Group: "", Version: "v1", Resource: "pods",
		},
		RequestSubResource: "status",
		Name:               "nginx",
		Namespace:          "default",
		Operation:          operation,
		UserInfo: authenticationv1.UserInfo{
			Username: "alice",
			UID:      "user-uid",
			Groups:   []string{"system:authenticated"},
			Extra: map[string]authenticationv1.ExtraValue{
				"reason": {"testing"},
			},
		},
		DryRun: &dryRun,
		Options: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"meta.k8s.io/v1","kind":"CreateOptions"}`),
		},
	}
	if objRaw != nil {
		req.Object = runtime.RawExtension{Raw: objRaw}
	}
	if oldObjRaw != nil {
		req.OldObject = runtime.RawExtension{Raw: oldObjRaw}
	}
	return req
}

// minimalRequest returns an AdmissionRequest with every optional field left
// at its zero value, to exercise the omitempty/null side of the contract.
func minimalRequest(operation admissionv1.Operation, objRaw, oldObjRaw []byte) *admissionv1.AdmissionRequest {
	req := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: operation,
		UserInfo:  authenticationv1.UserInfo{},
	}
	if objRaw != nil {
		req.Object = runtime.RawExtension{Raw: objRaw}
	}
	if oldObjRaw != nil {
		req.OldObject = runtime.RawExtension{Raw: oldObjRaw}
	}
	return req
}

func requestMapTestCases() []struct {
	name      string
	request   *admissionv1.AdmissionRequest
	objRaw    []byte
	oldObjRaw []byte
} {
	return []struct {
		name      string
		request   *admissionv1.AdmissionRequest
		objRaw    []byte
		oldObjRaw []byte
	}{
		{
			name:    "CREATE - fully populated optional fields",
			request: baseRequest(admissionv1.Create, podRaw("nginx", "nginx:1.21"), nil),
			objRaw:  podRaw("nginx", "nginx:1.21"),
		},
		{
			name:    "CREATE - minimal/empty optional fields",
			request: minimalRequest(admissionv1.Create, podRaw("nginx", "nginx:1.21"), nil),
			objRaw:  podRaw("nginx", "nginx:1.21"),
		},
		{
			name:      "UPDATE - fully populated optional fields",
			request:   baseRequest(admissionv1.Update, podRaw("nginx", "nginx:1.22"), podRaw("nginx", "nginx:1.21")),
			objRaw:    podRaw("nginx", "nginx:1.22"),
			oldObjRaw: podRaw("nginx", "nginx:1.21"),
		},
		{
			name:      "UPDATE - minimal/empty optional fields",
			request:   minimalRequest(admissionv1.Update, podRaw("nginx", "nginx:1.22"), podRaw("nginx", "nginx:1.21")),
			objRaw:    podRaw("nginx", "nginx:1.22"),
			oldObjRaw: podRaw("nginx", "nginx:1.21"),
		},
		{
			name:      "DELETE - object empty, oldObject populated",
			request:   baseRequest(admissionv1.Delete, nil, podRaw("nginx", "nginx:1.21")),
			oldObjRaw: podRaw("nginx", "nginx:1.21"),
		},
		{
			name:      "DELETE - minimal optional fields",
			request:   minimalRequest(admissionv1.Delete, nil, podRaw("nginx", "nginx:1.21")),
			oldObjRaw: podRaw("nginx", "nginx:1.21"),
		},
		{
			name:    "CONNECT - both object and oldObject empty",
			request: baseRequest(admissionv1.Connect, nil, nil),
		},
		{
			name:    "CONNECT - minimal optional fields",
			request: minimalRequest(admissionv1.Connect, nil, nil),
		},
	}
}

// TestBuildRawRequestMap_GoldenEquality pins BuildRawRequestMap (vpol/gpol)
// against the literal base-2360fc7 formula: utils.ConvertObjectToUnstructured
// (request), verbatim, no splice.
func TestBuildRawRequestMap_GoldenEquality(t *testing.T) {
	for _, tc := range requestMapTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			oracle := rawVariantOracle(t, tc.request)

			got, err := BuildRawRequestMap(tc.request)
			require.NoError(t, err)
			assert.True(t, reflect.DeepEqual(oracle, got),
				"BuildRawRequestMap must equal base vpol/gpol's ConvertObjectToUnstructured(request) verbatim\noracle: %#v\ngot: %#v", oracle, got)
		})
	}
}

// TestBuildNormalizedRequestMap_GoldenEquality pins BuildNormalizedRequestMap
// (mpol) against the literal base-2360fc7 formula:
// utils.ConvertObjectToUnstructured(request) followed by the unconditional
// ExtractResources splice.
func TestBuildNormalizedRequestMap_GoldenEquality(t *testing.T) {
	for _, tc := range requestMapTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			oracle := normalizedVariantOracle(t, tc.request)

			got, err := BuildNormalizedRequestMap(tc.request)
			require.NoError(t, err)
			assert.True(t, reflect.DeepEqual(oracle, got),
				"BuildNormalizedRequestMap must equal base mpol's unconditional ExtractResources splice\noracle: %#v\ngot: %#v", oracle, got)
		})
	}
}

// TestBuildNormalizedRequestMapFromResources_SplicesSuppliedResources proves
// the read-only-path variant splices the resources the caller hands it
// rather than re-extracting from the request. This is the exact property the
// mutate-existing matching path relies on (engineImpl.MatchedMutateExisting-
// Policies reuses the object/oldObject it already extracted for attr instead
// of paying a second ExtractResources), and the regression this commit is
// vulnerable to: if the primitive ever self-extracts again, the sentinel
// below - which ExtractResources could never produce from the pod request -
// stops surfacing.
func TestBuildNormalizedRequestMapFromResources_SplicesSuppliedResources(t *testing.T) {
	req := minimalRequest(admissionv1.Create, podRaw("nginx", "nginx:1.21"), nil)

	objectSentinel := unstructured.Unstructured{Object: map[string]any{"sentinel": "object"}}
	oldObjectSentinel := unstructured.Unstructured{Object: map[string]any{"sentinel": "oldObject"}}

	got, err := BuildNormalizedRequestMapFromResources(req, objectSentinel, oldObjectSentinel)
	require.NoError(t, err)
	assert.Equal(t, objectSentinel.Object, got["object"],
		"FromResources must splice the supplied object, not re-extract from request")
	assert.Equal(t, oldObjectSentinel.Object, got["oldObject"],
		"FromResources must splice the supplied oldObject, not re-extract from request")
}

// TestBuildRawRequestMap_NumericFidelity specifically targets int64-vs-
// float64 drift: BuildRawRequestMap must decode integers the same way
// ConvertObjectToUnstructured already does at base.
func TestBuildRawRequestMap_NumericFidelity(t *testing.T) {
	raw := podRaw("nginx", "nginx:1.21")
	req := minimalRequest(admissionv1.Create, raw, nil)

	got, err := BuildRawRequestMap(req)
	require.NoError(t, err)

	objectMap, ok := got["object"].(map[string]any)
	require.True(t, ok, "object must be a map[string]any")

	metadata, ok := objectMap["metadata"].(map[string]any)
	require.True(t, ok)
	generation, ok := metadata["generation"]
	require.True(t, ok)
	assert.IsType(t, int64(0), generation, "integer fields must decode as int64, not float64")
	assert.Equal(t, int64(3), generation)

	spec, ok := objectMap["spec"].(map[string]any)
	require.True(t, ok)
	assert.IsType(t, int64(0), spec["terminationGracePeriodSeconds"])
	assert.Equal(t, int64(30), spec["terminationGracePeriodSeconds"])
	assert.IsType(t, int64(0), spec["priority"])
	assert.Equal(t, int64(-1), spec["priority"])
	assert.IsType(t, false, spec["hostNetwork"])
	assert.Equal(t, false, spec["hostNetwork"])

	containers, ok := spec["containers"].([]any)
	require.True(t, ok)
	require.Len(t, containers, 1)
	container, ok := containers[0].(map[string]any)
	require.True(t, ok)
	ports, ok := container["ports"].([]any)
	require.True(t, ok)
	require.Len(t, ports, 1)
	port, ok := ports[0].(map[string]any)
	require.True(t, ok)
	assert.IsType(t, int64(0), port["containerPort"])
	assert.Equal(t, int64(8080), port["containerPort"])
}

// TestBuildRequestMap_NilRequest documents that a nil request produces a nil
// map without error for both variants, matching the pre-existing behavior of
// the JSON/no-attributes evaluation path.
func TestBuildRequestMap_NilRequest(t *testing.T) {
	got, err := BuildRawRequestMap(nil)
	assert.NoError(t, err)
	assert.Nil(t, got)

	got, err = BuildNormalizedRequestMap(nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

// TestBuildRawRequestMap_NoNamespaceInjection is a mandatory flaw-pinning
// case (design.md's 2026-09-12 escalation amendment): a CREATE whose
// Object.Raw omits metadata.namespace, with request.Namespace set. Base
// vpol/gpol's request.object is a raw unmarshal of request.Object.Raw with
// no GVK/namespace normalization - has(request.object.metadata.namespace)
// must stay false. This is the exact case that flipped false->true under
// the withdrawn splice-always design and must never regress silently.
func TestBuildRawRequestMap_NoNamespaceInjection(t *testing.T) {
	rawWithoutNamespace := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx"},"spec":{}}`)
	req := minimalRequest(admissionv1.Create, rawWithoutNamespace, nil)
	req.Namespace = "default" // the request carries a namespace the raw object bytes omit

	got, err := BuildRawRequestMap(req)
	require.NoError(t, err)

	objectMap, ok := got["object"].(map[string]any)
	require.True(t, ok)
	metadata, ok := objectMap["metadata"].(map[string]any)
	require.True(t, ok)
	_, hasNamespace := metadata["namespace"]
	assert.False(t, hasNamespace,
		"BuildRawRequestMap must not inject request.Namespace into request.object.metadata - base vpol/gpol never normalized this")

	env, err := cel.NewEnv(cel.Variable("request", cel.DynType))
	require.NoError(t, err)
	ast, iss := env.Compile("has(request.object.metadata.namespace)")
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := prg.Eval(map[string]any{"request": got})
	require.NoError(t, err)
	result, err := out.ConvertToNative(reflect.TypeOf(false))
	require.NoError(t, err)
	assert.False(t, result.(bool), "has(request.object.metadata.namespace) must stay false for the raw variant")
}

// TestBuildNormalizedRequestMap_NamespaceInjected is the mpol-side
// counterpart: BuildNormalizedRequestMap DOES normalize (that's mpol's base
// behavior, unchanged), so the same namespace-omitting raw bytes end up with
// an injected namespace via ExtractResources.
func TestBuildNormalizedRequestMap_NamespaceInjected(t *testing.T) {
	rawWithoutNamespace := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx"},"spec":{}}`)
	req := minimalRequest(admissionv1.Create, rawWithoutNamespace, nil)
	req.Namespace = "default"

	got, err := BuildNormalizedRequestMap(req)
	require.NoError(t, err)

	objectMap, ok := got["object"].(map[string]any)
	require.True(t, ok)
	metadata, ok := objectMap["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "default", metadata["namespace"],
		"BuildNormalizedRequestMap must normalize via ExtractResources, injecting request.Namespace - this is mpol's unchanged base behavior")
}

// TestBuildRawRequestMap_EmptySide_IsCELNull is a mandatory flaw-pinning
// case: on a DELETE, base vpol/gpol's request.object is nil (from
// RawExtension{}.MarshalJSON() -> "null"), which CEL sees as null:
// has(request.object) is true and request.object == null is true.
func TestBuildRawRequestMap_EmptySide_IsCELNull(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("request", cel.DynType))
	require.NoError(t, err)
	ast, iss := env.Compile("has(request.object) && request.object == null")
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)

	req := minimalRequest(admissionv1.Delete, nil, podRaw("nginx", "nginx:1.21"))
	got, err := BuildRawRequestMap(req)
	require.NoError(t, err)
	require.Contains(t, got, "object", "the object key must be present even when empty")

	out, _, err := prg.Eval(map[string]any{"request": got})
	require.NoError(t, err)
	result, err := out.ConvertToNative(reflect.TypeOf(false))
	require.NoError(t, err)
	assert.True(t, result.(bool), "request.object must be CEL-null on the empty side for the raw variant, exactly as base vpol/gpol produced")
}

// TestBuildNormalizedRequestMap_EmptySide_IsNotCELNull is the mpol-side
// mandatory flaw-pinning case: base mpol unconditionally spliced
// ExtractResources' typed-nil-map object on the empty side, so
// request.object == null was FALSE on DELETE (CEL sees a present, non-null
// empty map). This must be preserved exactly - it is base behavior, not a
// bug this PR fixes.
func TestBuildNormalizedRequestMap_EmptySide_IsNotCELNull(t *testing.T) {
	env, err := cel.NewEnv(cel.Variable("request", cel.DynType))
	require.NoError(t, err)
	ast, iss := env.Compile("has(request.object) && request.object != null")
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast)
	require.NoError(t, err)

	req := minimalRequest(admissionv1.Delete, nil, podRaw("nginx", "nginx:1.21"))
	got, err := BuildNormalizedRequestMap(req)
	require.NoError(t, err)
	require.Contains(t, got, "object", "the object key must be present even when empty")

	out, _, err := prg.Eval(map[string]any{"request": got})
	require.NoError(t, err)
	result, err := out.ConvertToNative(reflect.TypeOf(false))
	require.NoError(t, err)
	assert.True(t, result.(bool),
		"request.object == null must stay FALSE on DELETE for the normalized variant - base mpol's typed-nil-map empty side, unchanged")
}
