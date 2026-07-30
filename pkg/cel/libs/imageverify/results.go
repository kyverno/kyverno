package imageverify

// ImageVerificationResults records, per image, whether a real cryptographic
// signature or attestation check succeeded during a single policy evaluation.
// It is the ImageValidatingPolicy analogue of v1's ImageVerificationMetadata
// (pkg/engine/api/imageverifymetadata.go), which serves the same purpose for
// ClusterPolicy.
//
// It exists because validationConfigurations.required cannot trust the result of
// a CEL expression: the expression is precisely what required is meant to
// distrust. The evidence is produced inside the verification functions and has to
// be read back afterwards, so it is recorded here instead.
//
// Only the verification CEL functions write to it, so a policy author cannot
// forge an entry. Both successful and failed attempts are recorded, so that an
// image no expression ever checked can be told apart from one that was checked
// and rejected.
//
// Once an image is verified it stays verified. That keeps the outcome independent
// of the order a policy's expressions happen to run in, and stops an expression
// from clearing a genuine verification by re-checking the same image against, for
// example, an empty attestor list.
type ImageVerificationResults struct {
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
	if r.verified == nil {
		r.verified = make(map[string]bool)
	}
	r.verified[image] = r.verified[image] || verified
}

// Status reports whether image passed verification, and whether it was checked at
// all. The two are separate because an image no expression ever tried to verify
// is a different failure from one that was checked and rejected, and the caller
// reports them with different messages.
func (r *ImageVerificationResults) Status(image string) (verified bool, attempted bool) {
	if r == nil {
		return false, false
	}
	v, ok := r.verified[image]
	return v, ok
}

// Reset discards all recorded results and must be called at the start of every
// evaluation. These results are bound into the CEL environment when the policy is
// compiled, so a compiled policy reused across resources would otherwise carry
// earlier verifications forward and admit an unverified image. Compilation is
// per-request today, but relying on that would make this fail open the moment a
// compiled policy cache is introduced.
func (r *ImageVerificationResults) Reset() {
	if r == nil {
		return
	}
	clear(r.verified)
}
