package deprecations

import (
	"sync"
	"sync/atomic"

	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// executionEscapeHatch is a per-binary, runtime-registered predicate: when set and it returns
// true, DenyLegacyWrite allows everything through. It exists so that a binary which still
// constructs legacy policy execution can honour the migration-grace toggle; a binary that never
// constructs legacy execution never registers it, so denial is unconditional there regardless of
// any flag or env var. See RegisterLegacyExecutionEscapeHatch.
var executionEscapeHatch atomic.Pointer[func() bool]

// RegisterLegacyExecutionEscapeHatch is called, at most once per binary, by the code that
// constructs legacy policy execution (the kyverno binary's resource-handler legacy branch over
// policycache.Cache, and the cleanup binary's legacy CleanupPolicy/ClusterCleanupPolicy
// controller). It must not be called by anything else: typed validation handlers, status
// writers, or any other legacy code that is not itself execution. Deleting or unwiring the
// execution constructor removes the call to this function, and with it the escape hatch, in
// that binary -- there is deliberately no unregister.
func RegisterLegacyExecutionEscapeHatch(f func() bool) {
	executionEscapeHatch.Store(&f)
}

// transitionalSubresources holds the subresources (currently only "status") that a legacy
// status writer still compiled into this binary has registered as safe to allow through the
// gate. It starts empty; pkg/deprecations itself registers nothing, so its own tests observe
// the post-#17710 state (every subresource denied) today.
var (
	transitionalSubresourcesMu sync.RWMutex
	transitionalSubresources   = sets.New[string]()
)

// AllowSubresourceForLegacyStatusWriter registers subresource as transitionally allowed on
// legacy kyverno.io policy kinds. Called only from init() in the dedicated legacy_status.go
// files that hold Kyverno's own legacy status writers; deleting a writer's file deletes its
// registration. There is deliberately no unregister API.
func AllowSubresourceForLegacyStatusWriter(subresource string) {
	transitionalSubresourcesMu.Lock()
	defer transitionalSubresourcesMu.Unlock()
	transitionalSubresources.Insert(subresource)
}

func subresourceAllowed(subresource string) bool {
	transitionalSubresourcesMu.RLock()
	defer transitionalSubresourcesMu.RUnlock()
	return transitionalSubresources.Has(subresource)
}

// Decision is the outcome of the legacy write gate for one admission request.
type Decision int

const (
	// Delegate means the gate has no opinion: the inner handler decides.
	Delegate Decision = iota
	// Deny means the request is a legacy policy write and must be refused.
	Deny
	// AllowRecovery means the request is the narrow finalizer-removal recovery update. The
	// caller must answer success directly and must not delegate: the inner handler is the
	// typed legacy validator, which would re-validate the unchanged spec and can refuse a
	// policy that no longer passes current validation, blocking recovery for exactly the
	// stale objects most likely to be stuck. It also disappears with the legacy types.
	AllowRecovery
)

// DenyLegacyWrite decides whether an admission request for a legacy kyverno.io policy kind must
// be hard-denied under the 1.20 write-time block on legacy policy APIs (see
// https://github.com/kyverno/kyverno/issues/17708). It reads only request metadata and, for the
// finalizer-removal carve-out, the raw object JSON -- no typed policy dependency. It returns
// (err, true) when the request is denied, and (nil, false) when it is allowed.
func DecideLegacyWrite(request admissionv1.AdmissionRequest) (Decision, error) {
	hatch := executionEscapeHatch.Load()
	var escapeHatch func() bool
	if hatch != nil {
		escapeHatch = *hatch
	}
	return decideLegacyWrite(request, escapeHatch, subresourceAllowed)
}

// denyLegacyWrite is the pure decision core: given an admission request and its two collaborators
// as explicit inputs, it has no package-level state and is exercised directly by table tests
// without touching the add-only registries.
func decideLegacyWrite(request admissionv1.AdmissionRequest, escapeHatch func() bool, subresourceAllowed func(string) bool) (Decision, error) {
	if escapeHatch != nil && escapeHatch() {
		return Delegate, nil
	}
	if !IsLegacyPolicyKind(request.Kind.Group, request.Kind.Kind) {
		return Delegate, nil
	}
	switch request.Operation {
	case admissionv1.Delete, admissionv1.Connect:
		return Delegate, nil
	case admissionv1.Create:
		// fall through to the deny below
	case admissionv1.Update:
		if request.SubResource != "" {
			if subresourceAllowed != nil && subresourceAllowed(request.SubResource) {
				return Delegate, nil
			}
			return deny(request)
		}
		if allowed, err := admissionutils.IsFinalizerRemovalOnTerminatingObject(request); err == nil && allowed {
			return AllowRecovery, nil
		}
		// A decode error from IsFinalizerRemovalOnTerminatingObject is deliberately not
		// surfaced as its own error: it falls through to the same standard deny message,
		// so a malformed request is denied for the same stated reason as any other blocked
		// write, not with a decode-internals leak.
	default:
		// Any other operation on a legacy kind (there is none known today) is denied by
		// default rather than allowed by omission.
	}
	return deny(request)
}

func deny(request admissionv1.AdmissionRequest) (Decision, error) {
	err, _ := BuildKindError(request.Kind.Group, request.Kind.Version, request.Kind.Kind)
	return Deny, err
}
