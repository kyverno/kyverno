package compiler

import (
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/sdk/extensions/cel/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apijson "k8s.io/apimachinery/pkg/util/json"
)

// blankCopyRequestMap runs the shared, cheap part of both request-map
// builders below: a value copy of *request with the (potentially large)
// Object/OldObject RawExtension payloads zeroed out before handing the copy
// to the existing SDK utils.ConvertObjectToUnstructured. With the payload
// blobs blanked, the reflective walk only touches cheap scalars/maps
// (kind, resource, userInfo, options, ...) and never re-marshals/
// re-unmarshals the (potentially large) admitted object bytes - that JSON
// round-trip is the expensive part both builders below eliminate. The
// zeroed RawExtensions marshal to null, so the returned map still carries
// "object"/"oldObject" keys with nil values, which each caller then
// overwrites per its own engine's semantics.
func blankCopyRequestMap(request *admissionv1.AdmissionRequest) (map[string]any, error) {
	copyReq := *request
	copyReq.Object = runtime.RawExtension{}
	copyReq.OldObject = runtime.RawExtension{}
	converted, err := utils.ConvertObjectToUnstructured(&copyReq)
	if err != nil {
		return nil, err
	}
	var requestMap map[string]any
	if converted != nil {
		requestMap = converted.Object
	}
	if requestMap == nil {
		requestMap = map[string]any{}
	}
	return requestMap, nil
}

// BuildRawRequestMap builds the CEL `request` activation value once per
// admission request. It is used by the ValidatingPolicy (vpol) and
// GeneratingPolicy (gpol) engines, which at base commit 2360fc7 never
// spliced object/oldObject into `request` at all - they built it via a
// single utils.ConvertObjectToUnstructured(request) call and left
// request.object/request.oldObject exactly what that produced. This
// function reproduces that: request.object/request.oldObject are plain,
// un-normalized, int64-preserving unmarshals of
// request.Object.Raw/request.OldObject.Raw - the same bytes admitted, with
// no GVK or namespace injection. An empty side (DELETE's object, CREATE's
// oldObject, CONNECT's both) stays nil, which CEL sees as null (has() is
// true, == null is true).
//
// Contrast with BuildNormalizedRequestMap (used by mpol): that variant
// deliberately normalizes and unconditionally splices, because that is what
// mpol did at base. The two engines never shared `request.object` semantics
// before this hoist, and must not be unified by it - see design.md's
// 2026-09-12 escalation amendment for the incident this asymmetry fixes.
//
// This must never route through admissionutils.ExtractResources: that
// helper (used to build the top-level `object`/`oldObject` CEL vars, and by
// BuildNormalizedRequestMap) forces request.Kind as the object's GVK and
// overwrites metadata.namespace with request.Namespace - a normalization
// vpol/gpol's request.object never had at base and must not gain here.
//
// The returned map is shared across every policy evaluated for a single
// admission request (vpol's per-policy loop in Handle, or gpol's single
// Evaluate call) and MUST be treated as immutable by every caller: no
// consumer may write into it, or into any of its nested maps/slices, after
// it is returned. CEL activations only ever read from it.
//
// Synthetic-request carve-out: callers that evaluate a synthesized
// AdmissionRequest (for example vpol's ExtractionMode pod-template
// evaluation, which builds a per-template synthRequest via
// extract.SynthesizePodAdmissionRequest) must NOT reuse the hoisted map
// built for the outer request - they must call BuildRawRequestMap again
// with the synthetic request to get a map whose object/oldObject reflect
// the synthesized resource rather than the original admitted one.
func BuildRawRequestMap(request *admissionv1.AdmissionRequest) (map[string]any, error) {
	if request == nil {
		return nil, nil
	}
	requestMap, err := blankCopyRequestMap(request)
	if err != nil {
		return nil, err
	}
	object, err := unmarshalRawExtension(request.Object)
	if err != nil {
		return nil, err
	}
	if object != nil {
		requestMap["object"] = object
	}
	oldObject, err := unmarshalRawExtension(request.OldObject)
	if err != nil {
		return nil, err
	}
	if oldObject != nil {
		requestMap["oldObject"] = oldObject
	}
	return requestMap, nil
}

// unmarshalRawExtension reproduces RawExtension.MarshalJSON's own precedence
// - Raw bytes win if present, else the typed Object field, else the JSON
// literal null - followed by an int64-preserving unmarshal via
// k8s.io/apimachinery/pkg/util/json. This mirrors exactly what
// utils.ConvertObjectToUnstructured(request) does internally when it hits a
// RawExtension field, so BuildRawRequestMap matches base byte-for-byte
// regardless of how the RawExtension was populated.
//
// Both forms occur in practice: a real AdmissionRequest off the webhook wire
// populates .Raw, but vpol's ExtractionMode synthesizes a per-pod-template
// AdmissionRequest via extract.SynthesizePodAdmissionRequest, which builds
// runtime.RawExtension{Object: attr.GetObject()} with .Raw left nil. A
// naive "only handle .Raw" implementation silently produces a null
// request.object for every extraction-mode evaluation instead of erroring,
// so this precedence must be replicated exactly, not approximated - see
// TestHandle_ExtractionMode_RequestObjectMatchesSynthesizedPod for the
// regression this closes.
func unmarshalRawExtension(re runtime.RawExtension) (any, error) {
	raw, err := re.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var value any
	if err := apijson.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// BuildNormalizedRequestMap builds the CEL `request` activation value once
// per admission request/Evaluate call. It is used by the MutatingPolicy
// (mpol) engine, which at base commit 2360fc7 unconditionally spliced
// admissionutils.ExtractResources(nil, *request)'s output into
// request.object/request.oldObject - the same GVK/namespace-normalized
// objects the top-level `object`/`oldObject` CEL vars use - regardless of
// whether a side was empty. This function reproduces that exactly: the
// splice happens UNCONDITIONALLY, so an empty side (for example DELETE's
// object) is a typed-nil map[string]any, which CEL sees as a present,
// non-null empty map (has() is true, == null is false) - not CEL's null.
//
// Contrast with BuildRawRequestMap (used by vpol/gpol): that variant never
// normalizes and leaves empty sides as CEL null. mpol never shared this
// semantic with vpol/gpol at base, and this hoist must not unify them - see
// design.md's 2026-09-12 escalation amendment for the incident this
// asymmetry fixes.
//
// This self-extracts from request (via ExtractResources) rather than taking
// pre-parsed objects, so the returned map owns its own object copies and
// never aliases the `attr`-derived object the mpol loop rebuilds with the
// patched resource after each policy - callers must not pass attr's objects
// into this builder, or the hoisted map would start reflecting per-policy
// mutable state instead of the loop-invariant original request. Read-only
// callers that have already extracted the resources should use
// BuildNormalizedRequestMapFromResources to avoid a second extraction; see
// its aliasing contract before doing so.
//
// The returned map is shared across every policy evaluated for a single
// mpol Handle()/Evaluate() call and MUST be treated as immutable by every
// caller: no consumer may write into it, or into any of its nested
// maps/slices, after it is returned. CEL activations only ever read from
// it; the only historical mutator was mpol's own splice, which now happens
// once inside this function on a map it owns.
func BuildNormalizedRequestMap(request *admissionv1.AdmissionRequest) (map[string]any, error) {
	if request == nil {
		return nil, nil
	}
	object, oldObject, err := admissionutils.ExtractResources(nil, *request)
	if err != nil {
		return nil, err
	}
	// Freshly extracted, owned copies - safe even on the mutating Handle
	// loop, which is why this self-extracting entry point exists.
	return BuildNormalizedRequestMapFromResources(request, object, oldObject)
}

// BuildNormalizedRequestMapFromResources is the read-only-path variant of
// BuildNormalizedRequestMap: it splices caller-supplied, already-extracted
// object/oldObject instead of re-running admissionutils.ExtractResources,
// so a caller that has already extracted the admission resources (for
// example to build an admission.Attributes) does not pay a second full
// unmarshal of the admitted-object bytes. The splice is unconditional,
// preserving mpol's typed-nil-map empty-side semantics exactly: an empty
// side (for example DELETE's object) carries a nil object.Object, which CEL
// sees as a present, non-null empty map, not null.
//
// Aliasing contract: the returned map ALIASES the provided resources -
// request.object/request.oldObject reference the same underlying maps as
// object.Object/oldObject.Object. This is only safe when those resources
// are never mutated after this map is built. It fits read-only
// match-condition evaluation (engineImpl.MatchedMutateExistingPolicies,
// whose attr is built once from these same resources and never rebuilt with
// a patched resource, and whose MatchesConditions path only reads). It MUST
// NOT be used on the mutating Handle loop, which rebuilds attr with each
// policy's patch after every iteration; there the alias would make the
// hoisted request map drift to per-policy mutable state. That path uses
// BuildNormalizedRequestMap, which self-extracts owned copies.
//
// Like BuildNormalizedRequestMap, the returned map is shared across every
// policy evaluated for a single request and MUST be treated as immutable by
// every consumer.
func BuildNormalizedRequestMapFromResources(request *admissionv1.AdmissionRequest, object, oldObject unstructured.Unstructured) (map[string]any, error) {
	if request == nil {
		return nil, nil
	}
	requestMap, err := blankCopyRequestMap(request)
	if err != nil {
		return nil, err
	}
	requestMap["object"] = object.Object
	requestMap["oldObject"] = oldObject.Object
	return requestMap, nil
}
