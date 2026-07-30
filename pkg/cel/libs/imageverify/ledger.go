package imageverify

// VerificationLedger records, per image, whether a real cryptographic signature
// or attestation check succeeded during a single policy evaluation.

// Only the verification CEL functions write to the ledger, so a policy author cannot forge an entry.
// The ledger records both successful and failed verification attempts to distinguish "never checked"
// from "checked and failed".
//
// Once an image is verified it stays verified, which keeps the outcome
// independent of the order a policy's expressions happen to run in, and stops an
// expression from clearing a genuine verification by re-checking the same image
// against (eg: empty attestor list).
type VerificationLedger struct {
	verified map[string]bool
}

func NewVerificationLedger() *VerificationLedger {
	return &VerificationLedger{verified: make(map[string]bool)}
}

// Record notes the outcome of a verification attempt for image. Callers must only
// report success when a cryptographic check genuinely passed, not when a CEL
// expression evaluated to true.
func (l *VerificationLedger) Record(image string, verified bool) {
	if l == nil {
		return
	}
	if l.verified == nil {
		l.verified = make(map[string]bool)
	}
	l.verified[image] = l.verified[image] || verified
}

// Status reports whether image passed verification, and whether it was checked at
// all. We are separating because an image no expression ever tried to
// verify is a different failure from one that was checked and rejected, and the
// caller reports them with different messages.
func (l *VerificationLedger) Status(image string) (verified bool, attempted bool) {
	if l == nil {
		return false, false
	}
	v, ok := l.verified[image]
	return v, ok
}

// Reset clears the ledger, and must be called at the start of every evaluation.
// The ledger is bound into the CEL environment when the policy is compiled, so a
// compiled policy that was reused across resources would otherwise carry earlier
// verifications forward and admit an unverified image. Compilation is per request
// today, but relying on that would make this fail open the moment a compiled
// policy cache is introduced.
func (l *VerificationLedger) Reset() {
	if l == nil {
		return
	}
	clear(l.verified)
}
