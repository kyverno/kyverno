package anchor

import (
	"errors"
	"fmt"
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
			if got := newNegationAnchorError(tt.args.msg); got != tt.want {
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
			if got := newConditionalAnchorError(tt.args.msg); got != tt.want {
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
			if got := newGlobalAnchorError(tt.args.msg); got != tt.want {
				t.Errorf("newGlobalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNegationAnchorError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "nil error",
		err:  nil,
		want: false,
	}, {
		name: "plain error with matching message should NOT be classified as anchor error",
		err:  errors.New("negation anchor matched in resource: test"),
		want: false,
	}, {
		name: "conditional anchor error is not negation",
		err:  newConditionalAnchorError("test"),
		want: false,
	}, {
		name: "global anchor error is not negation",
		err:  newGlobalAnchorError("test"),
		want: false,
	}, {
		name: "direct negation anchor error",
		err:  newNegationAnchorError("test"),
		want: true,
	}, {
		name: "wrapped negation anchor error should be recognized",
		err:  fmt.Errorf("wrapper: %w", newNegationAnchorError("test")),
		want: true,
	}, {
		name: "doubly wrapped negation anchor error should be recognized",
		err:  fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", newNegationAnchorError("test"))),
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNegationAnchorError(tt.err); got != tt.want {
				t.Errorf("IsNegationAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConditionalAnchorError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "nil error",
		err:  nil,
		want: false,
	}, {
		name: "plain error with matching message should NOT be classified as anchor error",
		err:  errors.New("conditional anchor mismatch: test"),
		want: false,
	}, {
		name: "direct conditional anchor error",
		err:  newConditionalAnchorError("test"),
		want: true,
	}, {
		name: "global anchor error is not conditional",
		err:  newGlobalAnchorError("test"),
		want: false,
	}, {
		name: "negation anchor error is not conditional",
		err:  newNegationAnchorError("test"),
		want: false,
	}, {
		name: "wrapped conditional anchor error should be recognized",
		err:  fmt.Errorf("wrapper: %w", newConditionalAnchorError("test")),
		want: true,
	}, {
		name: "unrelated error with same text should NOT be classified as anchor error",
		err:  fmt.Errorf("wrapper: %w", errors.New("conditional anchor mismatch: something")),
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConditionalAnchorError(tt.err); got != tt.want {
				t.Errorf("IsConditionalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGlobalAnchorError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{{
		name: "nil error",
		err:  nil,
		want: false,
	}, {
		name: "plain error with matching message should NOT be classified as anchor error",
		err:  errors.New("global anchor mismatch: test"),
		want: false,
	}, {
		name: "conditional anchor error is not global",
		err:  newConditionalAnchorError("test"),
		want: false,
	}, {
		name: "direct global anchor error",
		err:  newGlobalAnchorError("test"),
		want: true,
	}, {
		name: "negation anchor error is not global",
		err:  newNegationAnchorError("test"),
		want: false,
	}, {
		name: "wrapped global anchor error should be recognized",
		err:  fmt.Errorf("wrapper: %w", newGlobalAnchorError("test")),
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGlobalAnchorError(tt.err); got != tt.want {
				t.Errorf("IsGlobalAnchorError() = %v, want %v", got, tt.want)
			}
		})
	}
}