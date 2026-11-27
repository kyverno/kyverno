package compiler

import (
	"bytes"
	"context"
	gojson "encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"

	jsonpatchv5 "github.com/evanphx/json-patch/v5"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"google.golang.org/protobuf/types/known/structpb"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	admission "k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	patch "k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	"k8s.io/apiserver/pkg/cel/mutation"
	"k8s.io/apiserver/pkg/cel/mutation/dynamic"
	pointer "k8s.io/utils/ptr"
	kjson "sigs.k8s.io/json"
)

// JSONPatchCondition contains the inputs needed to compile and evaluate a cel expression
// that returns a JSON patch value.
type JSONPatchCondition struct {
	Expression string
}

var _ plugincel.ExpressionAccessor = &JSONPatchCondition{}

func (v *JSONPatchCondition) GetExpression() string {
	return v.Expression
}

func (v *JSONPatchCondition) ReturnTypes() []*cel.Type {
	return []*cel.Type{cel.ListType(jsonPatchType)}
}

var jsonPatchType = types.NewObjectType("JSONPatch")

const (
	maxJSONDocumentSize    = 1 << 20
	maxJSONPatchOperations = 1024
)

var (
	defaultObjectInterfaces = admission.NewObjectInterfacesFromScheme(runtime.NewScheme())
	defaultJSONSerializer   = json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		defaultObjectInterfaces.GetObjectCreater(),
		defaultObjectInterfaces.GetObjectTyper(),
		json.SerializerOptions{Pretty: false, Strict: true},
	)
)

// NewJSONPatcher creates a patcher that performs a JSON Patch mutation.
func newJSONPatcher(prog cel.Program) Patcher {
	return &jsonPatcher{
		prog:           prog,
		jsonSerializer: defaultJSONSerializer,
	}
}

type jsonPatcher struct {
	prog           cel.Program
	jsonSerializer *json.Serializer
}

func (e *jsonPatcher) Patch(ctx context.Context, evalData map[string]any, patchRequest patch.Request, runtimeCELCostBudget int64) (runtime.Object, MutationEval, error) {
	patchObj, _, eval, err := e.evaluatePatchExpression(ctx, runtimeCELCostBudget, evalData)
	if err != nil {
		return nil, eval, err
	}
	o := patchRequest.ObjectInterfaces
	if o == nil {
		o = defaultObjectInterfaces
	}
	jsonSerializer := e.jsonSerializer
	if jsonSerializer == nil {
		jsonSerializer = defaultJSONSerializer
	}
	if patchRequest.ObjectInterfaces != nil && patchRequest.ObjectInterfaces != defaultObjectInterfaces {
		jsonSerializer = json.NewSerializerWithOptions(json.DefaultMetaFactory, o.GetObjectCreater(), o.GetObjectTyper(), json.SerializerOptions{Pretty: false, Strict: true})
	}
	objJS, err := runtime.Encode(jsonSerializer, patchRequest.VersionedAttributes.VersionedObject.Object())
	if err != nil {
		return nil, eval, fmt.Errorf("failed to create JSON patch: %w", err)
	}
	patchedJS, err := patchObj.Apply(objJS)
	if err != nil {
		if errors.Is(err, jsonpatch.ErrTestFailed) {
			// If a json patch fails a test operation, the patch must not be applied
			eval.NotApplied = "a test operation failed, so the object was left unchanged"
			return patchRequest.VersionedAttributes.VersionedObject.Object(), eval, nil
		}
		return nil, eval, fmt.Errorf("JSON Patch: %w", err)
	}

	var newVersionedObject runtime.Object
	if _, ok := patchRequest.VersionedAttributes.VersionedObject.Object().(*unstructured.Unstructured); ok {
		newVersionedObject = &unstructured.Unstructured{}
	} else {
		newVersionedObject, err = o.GetObjectCreater().New(patchRequest.VersionedAttributes.VersionedKind)
		if err != nil {
			return nil, eval, apierrors.NewInternalError(err)
		}
	}

	if newVersionedObject, _, err = jsonSerializer.Decode(patchedJS, nil, newVersionedObject); err != nil {
		return nil, eval, apierrors.NewInternalError(err)
	}

	return newVersionedObject, eval, nil
}

func (e *jsonPatcher) PatchJSON(ctx context.Context, evalData map[string]any, runtimeCELCostBudget int64) (*unstructured.Unstructured, MutationEval, error) {
	object, ok := evalData[celcompiler.ObjectKey].(map[string]any)
	if !ok {
		return nil, MutationEval{}, fmt.Errorf("JSON Patch requires an object root")
	}
	encodable, _, err := keepDoubleFractions(object)
	if err != nil {
		return nil, MutationEval{}, fmt.Errorf("failed to encode JSON object: %w", err)
	}
	objJS, err := gojson.Marshal(encodable)
	if err != nil {
		return nil, MutationEval{}, fmt.Errorf("failed to encode JSON object: %w", err)
	}
	if err := checkJSONDocumentSize(len(objJS)); err != nil {
		return nil, MutationEval{}, err
	}
	patchObj, _, eval, err := e.evaluatePatchExpressionWithLimit(ctx, runtimeCELCostBudget, evalData, maxJSONPatchOperations, true)
	if err != nil {
		return nil, eval, err
	}
	patchedJS, err := applyJSONPatchWithLimits(objJS, patchObj)
	if errors.Is(err, jsonpatchv5.ErrTestFailed) {
		eval.NotApplied = "a test operation failed, so the object was left unchanged"
		return &unstructured.Unstructured{Object: object}, eval, nil
	}
	if err != nil {
		return nil, eval, fmt.Errorf("JSON Patch: %w", err)
	}
	var patched map[string]any
	if err := kjson.UnmarshalCaseSensitivePreserveInts(patchedJS, &patched); err != nil || patched == nil {
		if err == nil {
			err = fmt.Errorf("JSON mutation must preserve an object root")
		}
		return nil, eval, err
	}
	return &unstructured.Unstructured{Object: patched}, eval, nil
}

func (e *jsonPatcher) evaluatePatchExpression(ctx context.Context, remainingBudget int64, evalData map[string]any) (jsonpatch.Patch, int64, MutationEval, error) {
	return e.evaluatePatchExpressionWithLimit(ctx, remainingBudget, evalData, 0, false)
}

// keepDoubles is set for JSON-mode documents, which are decoded with preserved ints, so an
// integral double must keep its fraction to come back as a double.
func (e *jsonPatcher) evaluatePatchExpressionWithLimit(ctx context.Context, remainingBudget int64, evalData map[string]any, maxOperations int, keepDoubles bool) (jsonpatch.Patch, int64, MutationEval, error) {
	var err error

	refVal, _, err := e.prog.ContextEval(ctx, evalData)
	eval := MutationEval{Result: refVal}
	if err != nil {
		return nil, -1, eval, err
	}

	// the return type can be any valid CEL value.
	// Scalars, maps and lists are used to set the value when the path points to a field of that type.
	// ObjectVal is used when the path points to a struct. A map like "{"field1": 1, "fieldX": bool}" is not
	// possible in Kubernetes CEL because maps and lists may not have mixed types.

	iter, ok := refVal.(traits.Lister)
	if !ok {
		// Should never happen since compiler checks return type.
		return nil, -1, eval, fmt.Errorf("type mismatch: JSONPatchType.expression should evaluate to array")
	}
	result := jsonpatch.Patch{}
	for it := iter.Iterator(); it.HasNext() == types.True; {
		v := it.Next()
		if maxOperations > 0 && len(result) >= maxOperations {
			return nil, -1, eval, fmt.Errorf("JSON Patch exceeds the maximum of %d operations", maxOperations)
		}
		patchObj, err := v.ConvertToNative(reflect.TypeOf(&mutation.JSONPatchVal{}))
		if err != nil {
			// Should never happen since return type is checked by compiler.
			return nil, -1, eval, fmt.Errorf("type mismatch: JSONPatchType.expression should evaluate to array of JSONPatch: %w", err)
		}
		op, ok := patchObj.(*mutation.JSONPatchVal)
		if !ok {
			// Should never happen since return type is checked by compiler.
			return nil, -1, eval, fmt.Errorf("type mismatch: JSONPatchType.expression should evaluate to array of JSONPatch, got element of %T", patchObj)
		}

		// Construct a JSON Patch from the evaluated CEL expression
		resultOp := jsonpatch.Operation{}
		resultOp["op"] = pointer.To(gojson.RawMessage(strconv.Quote(op.Op)))
		resultOp["path"] = pointer.To(gojson.RawMessage(strconv.Quote(op.Path)))
		if op.Op == "copy" || op.Op == "move" {
			resultOp["from"] = pointer.To(gojson.RawMessage(strconv.Quote(op.From)))
		}
		if op.Val != nil {
			b, err := patchValueToJSON(op.Val, keepDoubles)
			if err != nil {
				return nil, -1, eval, err
			}
			resultOp["value"] = pointer.To[gojson.RawMessage](b)
		}

		result = append(result, resultOp)
	}

	return result, remainingBudget, eval, nil
}

func applyJSONPatchWithLimits(document []byte, patchObj jsonpatch.Patch) ([]byte, error) {
	rawPatchSize := 0
	for _, operation := range patchObj {
		for _, rawValue := range operation {
			if rawValue == nil {
				continue
			}
			if len(*rawValue) > maxJSONDocumentSize-rawPatchSize {
				return nil, fmt.Errorf("JSON Patch exceeds the maximum size of %d bytes", maxJSONDocumentSize)
			}
			rawPatchSize += len(*rawValue)
		}
	}

	patchJSON, err := gojson.Marshal(patchObj)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON Patch: %w", err)
	}
	if err := checkJSONDocumentSize(len(patchJSON)); err != nil {
		return nil, fmt.Errorf("JSON Patch: %w", err)
	}

	growthBudget := maxJSONDocumentSize - len(document)
	for _, operation := range patchObj {
		switch operation.Kind() {
		case "add", "replace", "copy", "move":
			encodedOperation, err := gojson.Marshal(operation)
			if err != nil {
				return nil, fmt.Errorf("failed to encode JSON Patch operation: %w", err)
			}
			growthBudget -= len(encodedOperation)
			if growthBudget < 0 {
				return nil, fmt.Errorf("JSON Patch may exceed the maximum document size of %d bytes", maxJSONDocumentSize)
			}
		}
	}
	for _, operation := range patchObj {
		if operation.Kind() == "copy" && growthBudget == 0 {
			return nil, fmt.Errorf("JSON Patch copy operations exceed the maximum document size of %d bytes", maxJSONDocumentSize)
		}
	}

	patchV5, err := jsonpatchv5.DecodePatch(patchJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON Patch: %w", err)
	}
	options := jsonpatchv5.NewApplyOptions()
	options.AccumulatedCopySizeLimit = int64(growthBudget)
	patched, err := patchV5.ApplyWithOptions(document, options)
	if err != nil {
		return nil, err
	}
	if err := checkJSONDocumentSize(len(patched)); err != nil {
		return nil, err
	}
	return patched, nil
}

func checkJSONDocumentSize(size int) error {
	if size > maxJSONDocumentSize {
		return fmt.Errorf("JSON document exceeds the maximum size of %d bytes", maxJSONDocumentSize)
	}
	return nil
}

// patchValueToJSON encodes the value of a single JSON Patch operation.
//
// Object initializers are converted through their native map rather than through
// structpb. ObjectVal.ConvertToNative answers a request for a *structpb.Value with a
// *structpb.Struct, which goes unnoticed for a bare object because the result is
// marshalled immediately and both encode to the same JSON.
//
// It does not go unnoticed once the object sits inside a container. cel-go converts a
// list to []*structpb.Value and a map to map[string]*structpb.Value, then fills each in
// by reflection, where the element type is enforced and the mismatch panics: Set for a
// list, SetMapIndex for a map. Both containers are therefore encoded entry by entry so
// that neither collection is ever built, which also makes the type-name check reachable
// for the objects inside them.
func patchValueToJSON(val ref.Val, keepDoubles bool) (gojson.RawMessage, error) {
	switch typed := val.(type) {
	case types.Int:
		return marshalPatchValue(int64(typed))
	case types.Uint:
		if uint64(typed) > math.MaxInt64 {
			return nil, fmt.Errorf("JSON Patch unsigned integer %d exceeds the int64 range", uint64(typed))
		}
		return marshalPatchValue(uint64(typed))
	case types.Double:
		if keepDoubles {
			b, err := marshalDouble(float64(typed))
			if err != nil {
				return nil, unmarshalableValue(err)
			}
			return b, nil
		}
		return marshalPatchValue(float64(typed))
	case *dynamic.ObjectVal:
		// TODO: Object initializers are insufficiently type checked.
		// In the interim, we use this sanity check to detect type mismatches
		// between field names and Object initializers. For example,
		// "Object.spec{ selector: Object.spec.wrong{}}" is detected as a mismatch.
		// Before beta, attaching full type information both to Object initializers and
		// the "object" and "oldObject" variables is needed. This will allow CEL to
		// perform comprehensive runtime type checking.
		if err := typed.CheckTypeNamesMatchFieldPathNames(); err != nil {
			return nil, fmt.Errorf("type mismatch: %w", err)
		}
		native, err := typed.ConvertToNative(reflect.TypeOf(map[string]any{}))
		if err != nil {
			return nil, unmarshalableValue(err)
		}
		if keepDoubles {
			if native, _, err = keepDoubleFractions(native); err != nil {
				return nil, unmarshalableValue(err)
			}
		}
		return marshalPatchValue(native)
	case traits.Lister:
		elements := []gojson.RawMessage{}
		for it := typed.Iterator(); it.HasNext() == types.True; {
			element, err := patchValueToJSON(it.Next(), keepDoubles)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)
		}
		return marshalPatchValue(elements)
	case traits.Mapper:
		entries := map[string]gojson.RawMessage{}
		for it := typed.Iterator(); it.HasNext() == types.True; {
			key := it.Next()
			name, ok := key.Value().(string)
			if !ok {
				return nil, fmt.Errorf("JSONPath valueExpression evaluated to a map with a %T key, only string keys can marshal to JSON", key.Value())
			}
			entry, err := patchValueToJSON(typed.Get(key), keepDoubles)
			if err != nil {
				return nil, err
			}
			entries[name] = entry
		}
		return marshalPatchValue(entries)
	default:
		// CEL data literals representing arbitrary JSON values can be serialized to JSON for use in
		// JSON Patch if first converted to pb.Value.
		v, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
		if err != nil {
			return nil, unmarshalableValue(err)
		}
		return marshalPatchValue(v)
	}
}

// marshalDouble encodes f like encoding/json, but appends ".0" to an integral value so
// that a decoder preserving ints reads it back as a double rather than an int64.
func marshalDouble(f float64) (gojson.RawMessage, error) {
	b, err := gojson.Marshal(f)
	if err != nil {
		return nil, err
	}
	if !bytes.ContainsAny(b, ".eE") {
		b = append(b, '.', '0')
	}
	return b, nil
}

// keepDoubleFractions returns v with every float64 replaced by its marshalDouble
// encoding. Containers are only copied when they hold a float64, so a document
// without doubles is returned as is.
func keepDoubleFractions(v any) (any, bool, error) {
	switch typed := v.(type) {
	case float64:
		b, err := marshalDouble(typed)
		return b, true, err
	case map[string]any:
		var out map[string]any
		for key, value := range typed {
			converted, changed, err := keepDoubleFractions(value)
			if err != nil {
				return nil, false, err
			}
			if !changed {
				continue
			}
			if out == nil {
				out = make(map[string]any, len(typed))
				for k, original := range typed {
					out[k] = original
				}
			}
			out[key] = converted
		}
		if out == nil {
			return v, false, nil
		}
		return out, true, nil
	case []any:
		var out []any
		for i, value := range typed {
			converted, changed, err := keepDoubleFractions(value)
			if err != nil {
				return nil, false, err
			}
			if !changed {
				continue
			}
			if out == nil {
				out = make([]any, len(typed))
				copy(out, typed)
			}
			out[i] = converted
		}
		if out == nil {
			return v, false, nil
		}
		return out, true, nil
	default:
		return v, false, nil
	}
}

func marshalPatchValue(v any) (gojson.RawMessage, error) {
	b, err := gojson.Marshal(v)
	if err != nil {
		return nil, unmarshalableValue(err)
	}
	return b, nil
}

func unmarshalableValue(err error) error {
	return fmt.Errorf("JSONPath valueExpression evaluated to a type that could not marshal to JSON: %w", err)
}
