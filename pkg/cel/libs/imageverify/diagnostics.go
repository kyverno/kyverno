package imageverify

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	diagnosticBudget    = 4 * 1024
	diagnosticPrefix    = "; verification details: "
	diagnosticTruncated = "; [verification details truncated]"
)

// verificationDiagnostics belongs to a single bound runtime. Evaluation is
// sequential within that runtime; concurrent requests bind separate instances.
type verificationDiagnostics struct {
	entries   []string
	size      int
	truncated bool
}

// BeginValidation discards diagnostics from earlier expressions. Cached CEL
// variables are not re-evaluated, so only new verification attempts are recorded.
func (r Runtime) BeginValidation() {
	if r.functions != nil && r.functions.diagnostics != nil {
		*r.functions.diagnostics = verificationDiagnostics{}
	}
}

// VerificationDiagnostics returns an immutable, bounded message suffix for the
// current validation. Snapshot it before evaluating messages or audit annotations.
func (r Runtime) VerificationDiagnostics() string {
	if r.functions == nil || r.functions.diagnostics == nil {
		return ""
	}
	d := r.functions.diagnostics
	if len(d.entries) == 0 {
		return ""
	}
	text := diagnosticPrefix + strings.Join(d.entries, "; ")
	if d.truncated {
		text += diagnosticTruncated
	}
	return text
}

func (d *verificationDiagnostics) record(image, attestor, attestation string, err error) {
	if d == nil || err == nil || d.truncated {
		return
	}
	text := fmt.Sprintf("image %q, attestor %q", image, attestor)
	if attestation != "" {
		text += fmt.Sprintf(", attestation %q", attestation)
	}
	text += ": " + strings.ToValidUTF8(err.Error(), "\uFFFD")
	for _, entry := range d.entries {
		if entry == text {
			return
		}
	}
	separator := 0
	if len(d.entries) > 0 {
		separator = len("; ")
	}
	remaining := diagnosticBudget - len(diagnosticPrefix) - len(diagnosticTruncated) - d.size - separator
	if len(text) > remaining {
		d.truncated = true
		if remaining <= 0 {
			return
		}
		text = text[:remaining]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
		// Do not retain the backing storage of an arbitrarily large error.
		text = strings.Clone(text)
	}
	d.entries = append(d.entries, text)
	d.size += separator + len(text)
}
