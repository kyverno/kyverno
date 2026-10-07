package deprecations

import (
	"context"

	"github.com/kyverno/kyverno/pkg/toggle"
	admissionv1 "k8s.io/api/admission/v1"
)

// ShouldBlock decides whether an admission request for a legacy kyverno.io policy kind must be
// hard-denied under the 1.20 write-time block on legacy policy APIs (see
// https://github.com/kyverno/kyverno/issues/17483). Only genuine creates and spec-changing
// updates are blocked:
//   - the toggle.BlockLegacyPolicyAPIs escape hatch, when disabled, allows everything through
//     (temporary 1.20 migration-grace opt-out, removed in 1.21)
//   - subresource requests (e.g. "status") are always allowed, so Kyverno's own controllers can
//     keep writing status on legacy policies they manage
//   - Delete/Connect operations, and any kind outside the legacy kyverno.io policy kinds, are
//     always allowed
//   - on Update, specsEqual is consulted to allow no-op re-applies (GitOps/Helm reconciliation
//     re-submitting an unchanged manifest, or a metadata/status-only change) through silently;
//     specsEqual is a thunk so callers only decode/compare the old and new specs when the
//     request could otherwise be blocked
//
// specsEqual is only invoked for Update requests on a legacy policy kind, once every cheaper
// check has already passed.
func ShouldBlock(ctx context.Context, request admissionv1.AdmissionRequest, specsEqual func() bool) (error, bool) {
	if !toggle.FromContext(ctx).BlockLegacyPolicyAPIs() {
		return nil, false
	}
	if request.SubResource != "" {
		return nil, false
	}
	if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
		return nil, false
	}
	if !IsLegacyPolicyKind(request.Kind.Group, request.Kind.Kind) {
		return nil, false
	}
	if request.Operation == admissionv1.Update && specsEqual() {
		return nil, false
	}
	err, _ := BuildKindError(request.Kind.Group, request.Kind.Version, request.Kind.Kind)
	return err, true
}
