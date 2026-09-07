package cosign

import "errors"

// SetupError marks a failure while preparing verification (CheckOpts, TUF/trust
// material, keys) rather than a genuine "not verified" result, so callers can
// surface it as an evaluation error subject to failurePolicy.
type SetupError struct{ Err error }

func (e *SetupError) Error() string { return e.Err.Error() }

func (e *SetupError) Unwrap() error { return e.Err }

// Setup wraps err as a SetupError. It returns nil when err is nil.
func Setup(err error) error {
	if err == nil {
		return nil
	}
	return &SetupError{Err: err}
}

// IsSetup reports whether err is (or wraps) a SetupError.
func IsSetup(err error) bool {
	var s *SetupError
	return errors.As(err, &s)
}
