package imageverify

import "sync"

// ImageVerificationResults records, per image, whether a real cryptographic
// signature or attestation check succeeded. IVPOL analogue of CPOL's
// ImageVerificationMetadata (pkg/engine/api/imageverifymetadata.go).
//
// required can't trust a CEL expression's return value (that's exactly what it
// exists to distrust) so verification functions record the real outcome here
// instead, for EnforceRequired to read back.
//
// One instance is shared by every policy compiled for the same admission request,
// so a wildcard required policy can see verifications done by other policies in
// that request. It's bound into the CEL environment at compile time, so a compiled
// policy belongs to that request and must not be cached/reused across requests.
//
// Only verification functions write to it (not exposed to CEL), and Record is
// monotonic so a later no-op check can't clear an earlier genuine verification.
// Both success and failure are recorded, to tell "never checked" apart from
// "checked and failed".
type ImageVerificationResults struct {
	mu       sync.RWMutex
	verified map[string]bool
}

func NewImageVerificationResults() *ImageVerificationResults {
	return &ImageVerificationResults{verified: make(map[string]bool)}
}

// Record notes the outcome of a verification attempt for image. Callers must only
// report success when a cryptographic check genuinely passed, not when a CEL
// expression evaluated to true.
func (r *ImageVerificationResults) Record(image string, verified bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.verified == nil {
		r.verified = make(map[string]bool)
	}
	r.verified[image] = r.verified[image] || verified
}

// Status reports whether image passed verification, and whether it was checked at
// all. The two are separate because an image no policy tried to verify is a
// different failure from one that was checked and rejected, and the caller
// reports them with different messages.
func (r *ImageVerificationResults) Status(image string) (verified bool, attempted bool) {
	if r == nil {
		return false, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.verified[image]
	return v, ok
}
