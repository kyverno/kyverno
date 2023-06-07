package pattern

import (
	"regexp"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"gotest.tools/assert"
)

func TestValidateValueWithFloatPattern_FloatValue(t *testing.T) {
	assert.Assert(t, validateFloatPattern(logr.Discard(), 7.9914, 7.9914))
}

func TestValidateValueWithFloatPattern_FloatValueNotPass(t *testing.T) {
	assert.Assert(t, !validateFloatPattern(logr.Discard(), 7.9914, 7.99141))
}

func TestValidateValueWithFloatPattern_FloatPatternWithoutFractionIntValue(t *testing.T) {
	assert.Assert(t, validateFloatPattern(logr.Discard(), 7, 7.000000))
}

func TestValidateValueWithFloatPattern_FloatPatternWithoutFraction(t *testing.T) {
	assert.Assert(t, validateFloatPattern(logr.Discard(), 7.000000, 7.000000))
}

func TestValidateValueWithIntPattern_FloatValueWithoutFraction(t *testing.T) {
	assert.Assert(t, validateFloatPattern(logr.Discard(), 7.000000, 7))
}

func TestValidateValueWithIntPattern_FloatValueWitFraction(t *testing.T) {
	assert.Assert(t, !validateFloatPattern(logr.Discard(), 7.000001, 7))
}

func TestValidateValueWithIntPattern_NotPass(t *testing.T) {
	assert.Assert(t, !validateFloatPattern(logr.Discard(), 8, 7))
}

func TestValidateValueWithStringPattern_WithSpace(t *testing.T) {
	assert.Assert(t, validateStringPattern(logr.Discard(), 4, ">= 3"))
}

func TestValidateValueWithStringPattern_Ranges(t *testing.T) {
	assert.Assert(t, validateStringPattern(logr.Discard(), 0, "0-2"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 1, "0-2"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 2, "0-2"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 3, "0-2"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 0, "10!-20"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 15, "10!-20"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 25, "10!-20"))

	assert.Assert(t, !validateStringPattern(logr.Discard(), 0, "0.00001-2.00001"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 1, "0.00001-2.00001"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 2, "0.00001-2.00001"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 2.0001, "0.00001-2.00001"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 0, "0.00001!-2.00001"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 1, "0.00001!-2.00001"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 2, "0.00001!-2.00001"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 2.0001, "0.00001!-2.00001"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 2, "2-2"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 2, "2!-2"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 2.99999, "2.99998-3"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 2.99997, "2.99998!-3"))
	assert.Assert(t, validateStringPattern(logr.Discard(), 3.00001, "2.99998!-3"))

	assert.Assert(t, validateStringPattern(logr.Discard(), "256Mi", "128Mi-512Mi"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), "1024Mi", "128Mi-512Mi"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), "64Mi", "128Mi-512Mi"))

	assert.Assert(t, !validateStringPattern(logr.Discard(), "256Mi", "128Mi!-512Mi"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "1024Mi", "128Mi!-512Mi"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "64Mi", "128Mi!-512Mi"))

	assert.Assert(t, validateStringPattern(logr.Discard(), -9, "-10-8"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), 9, "-10--8"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 9, "-10!--8"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "9Mi", "-10Mi!--8Mi"))

	assert.Assert(t, !validateStringPattern(logr.Discard(), -9, "-10!--8"))

	assert.Assert(t, validateStringPattern(logr.Discard(), "-9Mi", "-10Mi-8Mi"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "9Mi", "-10Mi!-8Mi"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 0, "-10-+8"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "7Mi", "-10Mi-+8Mi"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 10, "-10!-+8"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "10Mi", "-10Mi!-+8Mi"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 0, "+0-+1"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "10Mi", "+0Mi-+1024Mi"))

	assert.Assert(t, validateStringPattern(logr.Discard(), 10, "+0!-+1"))
	assert.Assert(t, validateStringPattern(logr.Discard(), "1025Mi", "+0Mi!-+1024Mi"))

}

func TestValidateNumberWithStr_LessFloatAndInt(t *testing.T) {
	assert.Assert(t, validateString(logr.Discard(), 7.00001, "7.000001", operator.More))
	assert.Assert(t, validateString(logr.Discard(), 7.00001, "7", operator.NotEqual))

	assert.Assert(t, validateString(logr.Discard(), 7.0000, "7", operator.Equal))
	assert.Assert(t, !validateString(logr.Discard(), 6.000000001, "6", operator.Less))
}

func TestValidateQuantity_InvalidQuantity(t *testing.T) {
	assert.Assert(t, !validateString(logr.Discard(), "1024Gi", "", operator.Equal))
	assert.Assert(t, !validateString(logr.Discard(), "gii", "1024Gi", operator.Equal))
}

func TestValidateDuration(t *testing.T) {
	assert.Assert(t, validateString(logr.Discard(), "12s", "12s", operator.Equal))
	assert.Assert(t, validateString(logr.Discard(), "12s", "15s", operator.NotEqual))
	assert.Assert(t, validateString(logr.Discard(), "12s", "15s", operator.Less))
	assert.Assert(t, validateString(logr.Discard(), "12s", "15s", operator.LessEqual))
	assert.Assert(t, validateString(logr.Discard(), "12s", "12s", operator.LessEqual))
	assert.Assert(t, !validateString(logr.Discard(), "15s", "12s", operator.Less))
	assert.Assert(t, !validateString(logr.Discard(), "15s", "12s", operator.LessEqual))
	assert.Assert(t, validateString(logr.Discard(), "15s", "12s", operator.More))
	assert.Assert(t, validateString(logr.Discard(), "15s", "12s", operator.MoreEqual))
	assert.Assert(t, validateString(logr.Discard(), "12s", "12s", operator.MoreEqual))
	assert.Assert(t, !validateString(logr.Discard(), "12s", "15s", operator.More))
	assert.Assert(t, !validateString(logr.Discard(), "12s", "15s", operator.MoreEqual))
}

func TestValidateQuantity_Equal(t *testing.T) {
	assert.Assert(t, validateString(logr.Discard(), "1024Gi", "1024Gi", operator.Equal))
	assert.Assert(t, validateString(logr.Discard(), "1024Mi", "1Gi", operator.Equal))
	assert.Assert(t, validateString(logr.Discard(), "0.2", "200m", operator.Equal))
	assert.Assert(t, validateString(logr.Discard(), "500", "500", operator.Equal))
	assert.Assert(t, !validateString(logr.Discard(), "2048", "1024", operator.Equal))
	assert.Assert(t, validateString(logr.Discard(), 1024, "1024", operator.Equal))
}

func TestValidateQuantity_Operation(t *testing.T) {
	assert.Assert(t, validateString(logr.Discard(), "1Gi", "1000Mi", operator.More))
	assert.Assert(t, validateString(logr.Discard(), "1G", "1Gi", operator.Less))
	assert.Assert(t, validateString(logr.Discard(), "500m", "0.5", operator.MoreEqual))
	assert.Assert(t, validateString(logr.Discard(), "1", "500m", operator.MoreEqual))
	assert.Assert(t, validateString(logr.Discard(), "0.5", ".5", operator.LessEqual))
	assert.Assert(t, validateString(logr.Discard(), "0.2", ".5", operator.LessEqual))
	assert.Assert(t, validateString(logr.Discard(), "0.2", ".5", operator.NotEqual))
}

func TestValidateQuantity_Operation_No_String_Check(t *testing.T) {
	log := funcr.New(
		func(prefix, args string) {
			assert.Assert(t, !strings.Contains(args, "Operators >, >=, <, <= are not applicable to strings"),
				"the compareString function should not be executed")
		},
		funcr.Options{Verbosity: 2},
	)
	assert.Assert(t, !validateString(log, "500m", "0.6", operator.MoreEqual))
}

func TestGetOperatorFromStringPattern_OneChar(t *testing.T) {
	assert.Equal(t, operator.GetOperatorFromStringPattern("f"), operator.Equal)
}

func TestGetOperatorFromStringPattern_EmptyString(t *testing.T) {
	assert.Equal(t, operator.GetOperatorFromStringPattern(""), operator.Equal)
}

func TestValidate(t *testing.T) {
	type args struct {
		value   interface{}
		pattern interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			value:   true,
			pattern: true,
		},
		want: true,
	}, {
		args: args{
			value:   true,
			pattern: false,
		},
		want: false,
	}, {
		args: args{
			value:   false,
			pattern: true,
		},
		want: false,
	}, {
		args: args{
			value:   false,
			pattern: false,
		},
		want: true,
	}, {
		args: args{
			value:   "value",
			pattern: nil,
		},
		want: false,
	}, {
		args: args{
			value:   "",
			pattern: nil,
		},
		want: true,
	}, {
		args: args{
			value:   0.0,
			pattern: nil,
		},
		want: true,
	}, {
		args: args{
			value:   0,
			pattern: nil,
		},
		want: true,
	}, {
		args: args{
			value:   false,
			pattern: nil,
		},
		want: true,
	}, {
		args: args{
			value:   "10.100.11.54",
			pattern: "192.168.88.1 | 10.100.11.*",
		},
		want: true,
	}, {
		args: args{
			value:   "10",
			pattern: ">1 & <20",
		},
		want: true,
	}, {
		args: args{
			value:   "10",
			pattern: ">1 & <20 | >31 & <33",
		},
		want: true,
	}, {
		args: args{
			value:   "32",
			pattern: ">1 & <20 | >31 & <33",
		},
		want: true,
	}, {
		args: args{
			value:   "21",
			pattern: ">1 & <20 | >31 & <33",
		},
		want: false,
	}, {
		args: args{
			value:   7.0,
			pattern: 7.000,
		},
		want: true,
	}, {
		args: args{
			value:   10,
			pattern: 10,
		},
		want: true,
	}, {
		args: args{
			value:   8,
			pattern: 10,
		},
		want: false,
	}, {
		args: args{
			value:   int64(10),
			pattern: int64(10),
		},
		want: true,
	}, {
		args: args{
			value:   int64(8),
			pattern: int64(10),
		},
		want: false,
	}, {
		args: args{
			value:   nil,
			pattern: []interface{}{},
		},
		want: false,
	}, {
		args: args{
			value:   nil,
			pattern: []string{},
		},
		want: false,
	}, {
		args: args{
			value: map[string]interface{}{
				"a": true,
			},
			pattern: map[string]interface{}{
				"a": true,
			},
		},
		want: true,
	}, {
		args: args{
			value: map[string]interface{}{
				"a": true,
			},
			pattern: map[string]interface{}{
				"b": false,
			},
		},
		want: true,
	}, {
		args: args{
			value:   nil,
			pattern: false,
		},
		want: false,
	}, {
		args: args{
			value:   8.0,
			pattern: 8,
		},
		want: true,
	}, {
		args: args{
			value:   8.1,
			pattern: 8,
		},
		want: false,
	}, {
		args: args{
			value:   "8",
			pattern: 8,
		},
		want: true,
	}, {
		args: args{
			value:   "8.1",
			pattern: 8,
		},
		want: false,
	}, {
		args: args{
			value:   false,
			pattern: 8,
		},
		want: false,
	}, {
		args: args{
			value:   8,
			pattern: 8.0,
		},
		want: true,
	}, {
		args: args{
			value:   8,
			pattern: 8.1,
		},
		want: false,
	}, {
		args: args{
			value:   int64(8),
			pattern: 8.0,
		},
		want: true,
	}, {
		args: args{
			value:   int64(8),
			pattern: 8.1,
		},
		want: false,
	}, {
		args: args{
			value:   "8",
			pattern: 8.0,
		},
		want: true,
	}, {
		args: args{
			value:   "8.1",
			pattern: 8.1,
		},
		want: true,
	}, {
		args: args{
			value:   "abc",
			pattern: 8.1,
		},
		want: false,
	}, {
		args: args{
			value:   false,
			pattern: 8.0,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Validate(logr.Discard(), tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("Validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_convertNumberToString(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{{
		args: args{
			value: nil,
		},
		want: "0",
	}, {
		args: args{
			value: "123",
		},
		want: "123",
	}, {
		args: args{
			value: "",
		},
		want: "",
	}, {
		args: args{
			value: "abc",
		},
		want: "abc",
	}, {
		args: args{
			value: 0.0,
		},
		want: "0.000000",
	}, {
		args: args{
			value: 3.10,
		},
		want: "3.100000",
	}, {
		args: args{
			value: -3.10,
		},
		want: "-3.100000",
	}, {
		args: args{
			value: -3,
		},
		want: "-3",
	}, {
		args: args{
			value: 3,
		},
		want: "3",
	}, {
		args: args{
			value: 0,
		},
		want: "0",
	}, {
		args: args{
			value: int64(-3),
		},
		want: "-3",
	}, {
		args: args{
			value: int64(3),
		},
		want: "3",
	}, {
		args: args{
			value: int64(0),
		},
		want: "0",
	}, {
		args: args{
			value: false,
		},
		wantErr: true,
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertNumberToString(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertNumberToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertNumberToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateMapPattern(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			value: nil,
		},
		want: false,
	}, {
		args: args{
			value: true,
		},
		want: false,
	}, {
		args: args{
			value: 8,
		},
		want: false,
	}, {
		args: args{
			value: "",
		},
		want: false,
	}, {
		args: args{
			value: "abc",
		},
		want: false,
	}, {
		args: args{
			value: map[string]interface{}(nil),
		},
		want: true,
	}, {
		args: args{
			value: map[string]interface{}{},
		},
		want: true,
	}, {
		args: args{
			value: map[string]interface{}{
				"a": true,
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateMapPattern(logr.Discard(), tt.args.value, nil); got != tt.want {
				t.Errorf("validateMapPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateNilPattern(t *testing.T) {
	type args struct {
		value interface{}
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		args: args{
			value: nil,
		},
		want: true,
	}, {
		args: args{
			value: 0.0,
		},
		want: true,
	}, {
		args: args{
			value: 0,
		},
		want: true,
	}, {
		args: args{
			value: int64(0),
		},
		want: true,
	}, {
		args: args{
			value: "",
		},
		want: true,
	}, {
		args: args{
			value: false,
		},
		want: true,
	}, {
		args: args{
			value: map[string]interface{}{},
		},
		want: false,
	}, {
		args: args{
			value: []interface{}{},
		},
		want: false,
	}, {
		args: args{
			value: map[string]string{},
		},
		want: false,
	}, {
		args: args{
			value: 1.0,
		},
		want: false,
	}, {
		args: args{
			value: 1,
		},
		want: false,
	}, {
		args: args{
			value: int64(1),
		},
		want: false,
	}, {
		args: args{
			value: "abc",
		},
		want: false,
	}, {
		args: args{
			value: true,
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateNilPattern(logr.Discard(), tt.args.value); got != tt.want {
				t.Errorf("validateNilPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateStringPatterns(t *testing.T) {
	type args struct {
		value   interface{}
		pattern string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{
				value:   "5.16.5-arch1-1",
				pattern: "!5.10.84-1",
			},
			want: true,
		},
		{
			args: args{
				value:   "5.10.84-1",
				pattern: "5.10.84-1",
			},
			want: true,
		},
		{
			args: args{
				value:   "!5.10.84-1",
				pattern: "!5.10.84-1",
			},
			want: true,
		},
		{
			args: args{
				value:   "5.10.84-1",
				pattern: "!5.10.84-1",
			},
			want: false,
		},
		{
			args: args{
				value:   "5.16.5-arch1-1",
				pattern: "!5.10.84-1 & !5.15.2-1",
			},
			want: true,
		},
		{
			args: args{
				value:   "5.10.84-1",
				pattern: "!5.10.84-1 & !5.15.2-1",
			},
			want: false,
		},
		{
			args: args{
				value:   "5.15.2-1",
				pattern: "!5.10.84-1 & !5.15.2-1",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateStringPatterns(logr.Discard(), tt.args.value, tt.args.pattern); got != tt.want {
				t.Errorf("validateStringPatterns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_split(t *testing.T) {
	type args struct {
		pattern string
		r       *regexp.Regexp
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 string
		want2 bool
	}{{
		args: args{
			pattern: "",
			r:       operator.InRangeRegex,
		},
		want:  "",
		want1: "",
		want2: false,
	}, {
		args: args{
			pattern: "",
			r:       operator.NotInRangeRegex,
		},
		want:  "",
		want1: "",
		want2: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := split(tt.args.pattern, tt.args.r)
			if got != tt.want {
				t.Errorf("split() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("split() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("split() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func Test_compareString(t *testing.T) {
	type args struct {
		value            interface{}
		pattern          string
		operatorVariable operator.Operator
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{
				value:            "anything",
				pattern:          "*",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "",
				pattern:          "*",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "leftright",
				pattern:          "*right",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "right",
				pattern:          "*right",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "leftmiddle",
				pattern:          "*right",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            "middle",
				pattern:          "*right",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            "abbeba",
				pattern:          "ab*ba",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "abbca",
				pattern:          "ab*ba",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            "abbba",
				pattern:          "ab?ba",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            "abbbba",
				pattern:          "ab?ba",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            nil,
				pattern:          "ab?ba",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            "",
				pattern:          "ab?ba",
				operatorVariable: operator.Equal,
			},
			want: false,
		}, {
			args: args{
				value:            "",
				pattern:          "",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            1,
				pattern:          "1",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            int64(1),
				pattern:          "1",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            1.0,
				pattern:          "1E+*",
				operatorVariable: operator.Equal,
			},
			want: true,
		}, {
			args: args{
				value:            true,
				pattern:          "true",
				operatorVariable: operator.Equal,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareString(logr.Discard(), tt.args.value, tt.args.pattern, tt.args.operatorVariable); got != tt.want {
				t.Errorf("compareString() = %v, want %v", got, tt.want)
			}
		})
	}
}
