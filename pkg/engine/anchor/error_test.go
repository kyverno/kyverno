package anchor

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// maskedError hides the wrapped error message while preserving its error chain.
type maskedError struct {
	err error
}

func (e maskedError) Error() string {
	return "masked error"
}

func (e maskedError) Unwrap() error {
	return e.err
}

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

func TestIsAnchorErrorWrapped(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isTarget func(error) bool
	}{
		{
			name:     "negation",
			err:      newNegationAnchorError("test"),
			isTarget: IsNegationAnchorError,
		},
		{
			name:     "conditional",
			err:      newConditionalAnchorError("test"),
			isTarget: IsConditionalAnchorError,
		},
		{
			name:     "global",
			err:      newGlobalAnchorError("test"),
			isTarget: IsGlobalAnchorError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("wrapped: %w", tt.err)
			if !tt.isTarget(wrapped) {
				t.Errorf("wrapped error was not classified as a %s anchor error", tt.name)
			}

			masked := maskedError{err: tt.err}
			if !tt.isTarget(masked) {
				t.Errorf("masked error was not classified as a %s anchor error", tt.name)
			}

			multiplyWrapped := fmt.Errorf("wrapped again: %w", masked)
			if !tt.isTarget(multiplyWrapped) {
				t.Errorf("multiply wrapped error was not classified as a %s anchor error", tt.name)
			}
		})
	}
}
