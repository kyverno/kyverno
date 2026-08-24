package anchor

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func Test_validateAnchorError_Error(t *testing.T) {
	type fields struct {
		err     anchorError
		message string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{{
		fields: fields{
			err:     negationAnchorErr,
			message: "test",
		},
		want: "test",
	}, {
		fields: fields{
			err:     conditionalAnchorErr,
			message: "test",
		},
		want: "test",
	}, {
		fields: fields{
			err:     globalAnchorErr,
			message: "test",
		},
		want: "test",
	}, {
		fields: fields{
			err:     globalAnchorErr,
			message: "",
		},
		want: "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := validateAnchorError{
				err:     tt.fields.err,
				message: tt.fields.message,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("validateAnchorError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newNegationAnchorError(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want validateAnchorError
	}{{
		args: args{
			msg: "test",
		},
		want: validateAnchorError{
			err:     negationAnchorErr,
			message: "negation anchor matched in resource: test",
		},
	}, {
		want: validateAnchorError{
			err:     negationAnchorErr,
			message: "negation anchor matched in resource: ",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newNegationAnchorError(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newNegationAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newConditionalAnchorError(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want validateAnchorError
	}{{
		args: args{
			msg: "test",
		},
		want: validateAnchorError{
			err:     conditionalAnchorErr,
			message: "conditional anchor mismatch: test",
		},
	}, {
		want: validateAnchorError{
			err:     conditionalAnchorErr,
			message: "conditional anchor mismatch: ",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newConditionalAnchorError(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newConditionalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newGlobalAnchorError(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want validateAnchorError
	}{{
		args: args{
			msg: "test",
		},
		want: validateAnchorError{
			err:     globalAnchorErr,
			message: "global anchor mismatch: test",
		},
	}, {
		want: validateAnchorError{
			err:     globalAnchorErr,
			message: "global anchor mismatch: ",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newGlobalAnchorError(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newGlobalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNegationAnchorError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			err: nil,
		},
		want: false,
	}, {
		args: args{
			err: errors.New("negation anchor matched in resource: test"),
		},
		want: false,
	}, {
		args: args{
			err: newConditionalAnchorError("test"),
		},
		want: false,
	}, {
		args: args{
			err: newGlobalAnchorError("test"),
		},
		want: false,
	}, {
		args: args{
			err: newNegationAnchorError("test"),
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNegationAnchorError(tt.args.err); got != tt.want {
				t.Errorf("IsNegationAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConditionalAnchorError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			err: nil,
		},
		want: false,
	}, {
		args: args{
			err: errors.New("conditional anchor mismatch: test"),
		},
		want: false,
	}, {
		args: args{
			err: newConditionalAnchorError("test"),
		},
		want: true,
	}, {
		args: args{
			err: newGlobalAnchorError("test"),
		},
		want: false,
	}, {
		args: args{
			err: newNegationAnchorError("test"),
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConditionalAnchorError(tt.args.err); got != tt.want {
				t.Errorf("IsConditionalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGlobalAnchorError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			err: nil,
		},
		want: false,
	}, {
		args: args{
			err: errors.New("global anchor mismatch: test"),
		},
		want: false,
	}, {
		args: args{
			err: newConditionalAnchorError("test"),
		},
		want: false,
	}, {
		args: args{
			err: newGlobalAnchorError("test"),
		},
		want: true,
	}, {
		args: args{
			err: newNegationAnchorError("test"),
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGlobalAnchorError(tt.args.err); got != tt.want {
				t.Errorf("IsGlobalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// maskedError wraps an error while deliberately replacing the rendered message.
// It models an error chain where the anchor error text is no longer visible, so
// only type-based classification can succeed.
type maskedError struct {
	inner error
}

func (m maskedError) Error() string { return "an internal error occurred" }

func (m maskedError) Unwrap() error { return m.inner }

// Test_isError_doesNotClassifyByMessage is the regression test for the removed
// strings.Contains fallback. Unrelated errors whose text happens to contain an
// anchor error message must never be classified as anchor errors, because that
// text can originate from resource content (see validateResourceElement, which
// formats resource values into its error messages).
func Test_isError_doesNotClassifyByMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{{
		name: "conditional message, unrelated error",
		err:  errors.New("conditional anchor mismatch: unrelated error"),
	}, {
		name: "global message, unrelated error",
		err:  errors.New("global anchor mismatch: unrelated error"),
	}, {
		name: "negation message, unrelated error",
		err:  errors.New("negation anchor matched in resource: unrelated error"),
	}, {
		name: "anchor message embedded in a resource value",
		err:  errors.New("resource value 'conditional anchor mismatch' does not match 'platform' at path /metadata/annotations/owner/"),
	}, {
		name: "anchor message as a substring",
		err:  errors.New("x-global anchor mismatch-y"),
	}, {
		name: "anchor message wrapped in an unrelated error",
		err:  fmt.Errorf("failed to validate: %w", errors.New("negation anchor matched in resource: test")),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConditionalAnchorError(tt.err); got {
				t.Errorf("IsConditionalAnchorError() = %v, want false", got)
			}
			if got := IsGlobalAnchorError(tt.err); got {
				t.Errorf("IsGlobalAnchorError() = %v, want false", got)
			}
			if got := IsNegationAnchorError(tt.err); got {
				t.Errorf("IsNegationAnchorError() = %v, want false", got)
			}
		})
	}
}

// Test_isError_classifiesWrappedAnchorErrors verifies that classification walks
// the error chain. Anchor errors are routinely aggregated by the validate
// package (multierr.Combine inside a *PatternError) before reaching skip() and
// fail(), so they are seldom directly type-assertable at the call site.
func Test_isError_classifiesWrappedAnchorErrors(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		wantCondition bool
		wantGlobal    bool
		wantNegation  bool
	}{{
		name:          "conditional, direct",
		err:           newConditionalAnchorError("test"),
		wantCondition: true,
	}, {
		name:          "conditional, wrapped with %w",
		err:           fmt.Errorf("context: %w", newConditionalAnchorError("test")),
		wantCondition: true,
	}, {
		name:          "conditional, wrapped and message masked",
		err:           maskedError{inner: newConditionalAnchorError("test")},
		wantCondition: true,
	}, {
		name:          "conditional, joined with an unrelated error",
		err:           errors.Join(errors.New("unrelated"), newConditionalAnchorError("test")),
		wantCondition: true,
	}, {
		name:          "conditional, nested two levels deep",
		err:           fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", newConditionalAnchorError("test"))),
		wantCondition: true,
	}, {
		name:       "global, direct",
		err:        newGlobalAnchorError("test"),
		wantGlobal: true,
	}, {
		name:       "global, wrapped with %w",
		err:        fmt.Errorf("context: %w", newGlobalAnchorError("test")),
		wantGlobal: true,
	}, {
		name:       "global, wrapped and message masked",
		err:        maskedError{inner: newGlobalAnchorError("test")},
		wantGlobal: true,
	}, {
		name:       "global, joined with an unrelated error",
		err:        errors.Join(errors.New("unrelated"), newGlobalAnchorError("test")),
		wantGlobal: true,
	}, {
		name:         "negation, direct",
		err:          newNegationAnchorError("test"),
		wantNegation: true,
	}, {
		name:         "negation, wrapped with %w",
		err:          fmt.Errorf("context: %w", newNegationAnchorError("test")),
		wantNegation: true,
	}, {
		name:         "negation, wrapped and message masked",
		err:          maskedError{inner: newNegationAnchorError("test")},
		wantNegation: true,
	}, {
		name:         "negation, joined with an unrelated error",
		err:          errors.Join(errors.New("unrelated"), newNegationAnchorError("test")),
		wantNegation: true,
	}, {
		name: "nil error",
		err:  nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConditionalAnchorError(tt.err); got != tt.wantCondition {
				t.Errorf("IsConditionalAnchorError() = %v, want %v", got, tt.wantCondition)
			}
			if got := IsGlobalAnchorError(tt.err); got != tt.wantGlobal {
				t.Errorf("IsGlobalAnchorError() = %v, want %v", got, tt.wantGlobal)
			}
			if got := IsNegationAnchorError(tt.err); got != tt.wantNegation {
				t.Errorf("IsNegationAnchorError() = %v, want %v", got, tt.wantNegation)
			}
		})
	}
}

// Test_wrapValidateAnchorError checks that the wrapping constructors preserve
// the cause for errors.As traversal while rendering exactly the same message as
// the string based constructors, so downstream messages are unchanged.
func Test_wrapValidateAnchorError(t *testing.T) {
	cause := errors.New("resource value does not match")

	tests := []struct {
		name    string
		got     validateAnchorError
		want    validateAnchorError
		wantMsg string
	}{{
		name:    "conditional",
		got:     wrapConditionalAnchorError(cause),
		want:    newConditionalAnchorError(cause.Error()),
		wantMsg: "conditional anchor mismatch: resource value does not match",
	}, {
		name:    "global",
		got:     wrapGlobalAnchorError(cause),
		want:    newGlobalAnchorError(cause.Error()),
		wantMsg: "global anchor mismatch: resource value does not match",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// message must be identical to the non wrapping constructor
			if tt.got.Error() != tt.want.Error() {
				t.Errorf("Error() = %v, want %v", tt.got.Error(), tt.want.Error())
			}
			if tt.got.Error() != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", tt.got.Error(), tt.wantMsg)
			}
			// the anchor type must be preserved
			if tt.got.err != tt.want.err {
				t.Errorf("err = %v, want %v", tt.got.err, tt.want.err)
			}
			// the cause must stay reachable
			if !errors.Is(tt.got, cause) {
				t.Errorf("errors.Is(got, cause) = false, want true")
			}
			if got := tt.got.Unwrap(); got != cause {
				t.Errorf("Unwrap() = %v, want %v", got, cause)
			}
		})
	}
}

// Test_validateAnchorError_Unwrap checks that anchor errors built without a
// cause report no cause, so errors.As stops cleanly at them.
func Test_validateAnchorError_Unwrap(t *testing.T) {
	tests := []struct {
		name string
		err  validateAnchorError
	}{
		{name: "negation", err: newNegationAnchorError("test")},
		{name: "conditional", err: newConditionalAnchorError("test")},
		{name: "global", err: newGlobalAnchorError("test")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Unwrap(); got != nil {
				t.Errorf("Unwrap() = %v, want nil", got)
			}
		})
	}
}

// Test_isError_distinguishesAnchorTypes guards the internal anchorError enum:
// each anchor type must remain distinguishable from the other two, including
// when the error is wrapped.
func Test_isError_distinguishesAnchorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  validateAnchorError
		want anchorError
	}{
		{name: "conditional", err: newConditionalAnchorError("test"), want: conditionalAnchorErr},
		{name: "global", err: newGlobalAnchorError("test"), want: globalAnchorErr},
		{name: "negation", err: newNegationAnchorError("test"), want: negationAnchorErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := []error{tt.err, fmt.Errorf("ctx: %w", tt.err), maskedError{inner: tt.err}}
			codes := []anchorError{conditionalAnchorErr, globalAnchorErr, negationAnchorErr}
			for _, e := range wrapped {
				for _, code := range codes {
					want := code == tt.want
					if got := isError(e, code); got != want {
						t.Errorf("isError(%v, %v) = %v, want %v", e, code, got, want)
					}
				}
			}
		})
	}
}

// Test_isError_outermostAnchorErrorWins documents the classification semantics
// when anchor errors are nested: the outermost anchor error decides. This is
// what conditionAnchorHandler relies on when it re-classifies a nested failure
// as a conditional anchor mismatch.
func Test_isError_outermostAnchorErrorWins(t *testing.T) {
	err := wrapConditionalAnchorError(newGlobalAnchorError("inner"))

	if !IsConditionalAnchorError(err) {
		t.Errorf("IsConditionalAnchorError() = false, want true")
	}
	if IsGlobalAnchorError(err) {
		t.Errorf("IsGlobalAnchorError() = true, want false (outermost anchor error wins)")
	}
}

// Test_isError_classifiesMultiplyWrappedAnchorErrors covers anchor errors buried
// several layers deep behind a mix of wrapping mechanisms: fmt.Errorf("%w"),
// errors.Join and a wrapper that replaces the message entirely. None of these
// could be recognised while classification was based on message text.
func Test_isError_classifiesMultiplyWrappedAnchorErrors(t *testing.T) {
	// bury wraps err behind three layers, the middle one hiding the message.
	bury := func(err error) error {
		return fmt.Errorf("outer: %w", maskedError{
			inner: errors.Join(errors.New("unrelated sibling"), fmt.Errorf("inner: %w", err)),
		})
	}

	tests := []struct {
		name          string
		err           error
		wantCondition bool
		wantGlobal    bool
		wantNegation  bool
	}{{
		name:          "conditional, multiply wrapped",
		err:           bury(newConditionalAnchorError("test")),
		wantCondition: true,
	}, {
		name:       "global, multiply wrapped",
		err:        bury(newGlobalAnchorError("test")),
		wantGlobal: true,
	}, {
		name:         "negation, multiply wrapped",
		err:          bury(newNegationAnchorError("test")),
		wantNegation: true,
	}, {
		name: "unrelated error, multiply wrapped",
		err:  bury(errors.New("conditional anchor mismatch: unrelated error")),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConditionalAnchorError(tt.err); got != tt.wantCondition {
				t.Errorf("IsConditionalAnchorError() = %v, want %v", got, tt.wantCondition)
			}
			if got := IsGlobalAnchorError(tt.err); got != tt.wantGlobal {
				t.Errorf("IsGlobalAnchorError() = %v, want %v", got, tt.wantGlobal)
			}
			if got := IsNegationAnchorError(tt.err); got != tt.wantNegation {
				t.Errorf("IsNegationAnchorError() = %v, want %v", got, tt.wantNegation)
			}
		})
	}
}
