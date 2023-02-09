package pattern

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/operator"
	"gotest.tools/assert"
)

func TestValidateString_AsteriskTest(t *testing.T) {
	pattern := "*"
	value := "anything"
	empty := ""

	assert.Assert(t, compareString(logr.Discard(), value, pattern, operator.Equal))
	assert.Assert(t, compareString(logr.Discard(), empty, pattern, operator.Equal))
}

func TestValidateString_LeftAsteriskTest(t *testing.T) {
	pattern := "*right"
	value := "leftright"
	right := "right"

	assert.Assert(t, compareString(logr.Discard(), value, pattern, operator.Equal))
	assert.Assert(t, compareString(logr.Discard(), right, pattern, operator.Equal))

	value = "leftmiddle"
	middle := "middle"

	assert.Assert(t, !compareString(logr.Discard(), value, pattern, operator.Equal))
	assert.Assert(t, !compareString(logr.Discard(), middle, pattern, operator.Equal))
}

func TestValidateString_MiddleAsteriskTest(t *testing.T) {
	pattern := "ab*ba"
	value := "abbeba"
	assert.Assert(t, compareString(logr.Discard(), value, pattern, operator.Equal))

	value = "abbca"
	assert.Assert(t, !compareString(logr.Discard(), value, pattern, operator.Equal))
}

func TestValidateString_QuestionMark(t *testing.T) {
	pattern := "ab?ba"
	value := "abbba"
	assert.Assert(t, compareString(logr.Discard(), value, pattern, operator.Equal))

	value = "abbbba"
	assert.Assert(t, !compareString(logr.Discard(), value, pattern, operator.Equal))
}

func TestValidateValueWithNilPattern_NullPatternStringValue(t *testing.T) {
	assert.Assert(t, !validateNilPattern(logr.Discard(), "value"))
}

func TestValidateValueWithNilPattern_NullPatternDefaultString(t *testing.T) {
	assert.Assert(t, validateNilPattern(logr.Discard(), ""))
}

func TestValidateValueWithNilPattern_NullPatternDefaultFloat(t *testing.T) {
	assert.Assert(t, validateNilPattern(logr.Discard(), 0.0))
}

func TestValidateValueWithNilPattern_NullPatternFloat(t *testing.T) {
	assert.Assert(t, !validateNilPattern(logr.Discard(), 0.1))
}

func TestValidateValueWithNilPattern_NullPatternDefaultInt(t *testing.T) {
	assert.Assert(t, validateNilPattern(logr.Discard(), 0))
}

func TestValidateValueWithNilPattern_NullPatternInt(t *testing.T) {
	assert.Assert(t, !validateNilPattern(logr.Discard(), 1))
}

func TestValidateValueWithNilPattern_NullPatternDefaultBool(t *testing.T) {
	assert.Assert(t, validateNilPattern(logr.Discard(), false))
}

func TestValidateValueWithNilPattern_NullPatternTrueBool(t *testing.T) {
	assert.Assert(t, !validateNilPattern(logr.Discard(), true))
}

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

func TestGetOperatorFromStringPattern_OneChar(t *testing.T) {
	assert.Equal(t, operator.GetOperatorFromStringPattern("f"), operator.Equal)
}

func TestGetOperatorFromStringPattern_EmptyString(t *testing.T) {
	assert.Equal(t, operator.GetOperatorFromStringPattern(""), operator.Equal)
}

func TestValidateKernelVersion_NotEquals(t *testing.T) {
	assert.Assert(t, validateStringPattern(logr.Discard(), "5.16.5-arch1-1", "!5.10.84-1"))
	assert.Assert(t, !validateStringPattern(logr.Discard(), "5.10.84-1", "!5.10.84-1"))
	assert.Assert(t, validateStringPatterns(logr.Discard(), "5.16.5-arch1-1", "!5.10.84-1 & !5.15.2-1"))
	assert.Assert(t, !validateStringPatterns(logr.Discard(), "5.10.84-1", "!5.10.84-1 & !5.15.2-1"))
	assert.Assert(t, !validateStringPatterns(logr.Discard(), "5.15.2-1", "!5.10.84-1 & !5.15.2-1"))
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
	},

		{
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
