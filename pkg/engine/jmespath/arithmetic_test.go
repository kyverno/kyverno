package jmespath

import (
	"reflect"
	"testing"

	"gotest.tools/assert"
)

func Test_Add(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Scalar
		{
			name:           "Scalar + Scalar -> Scalar",
			test:           "add(`12`, `13`)",
			expectedResult: 25.0,
			retFloat:       true,
		},
		{
			name: "Scalar + Duration -> error",
			test: "add('12', '13s')",
			err:  true,
		},
		{
			name: "Scalar + Quantity -> error",
			test: "add(`12`, '13Ki')",
			err:  true,
		},
		{
			name: "Scalar + Quantity -> error",
			test: "add(`12`, '13')",
			err:  true,
		},
		// Quantity
		{
			name:           "Quantity + Quantity -> Quantity",
			test:           "add('12Ki', '13Ki')",
			expectedResult: `25Ki`,
		},
		{
			name:           "Quantity + Quantity -> Quantity",
			test:           "add('12Ki', '13')",
			expectedResult: `12301`,
		},
		{
			name: "Quantity + Duration -> error",
			test: "add('12Ki', '13s')",
			err:  true,
		},
		{
			name: "Quantity + Scalar -> error",
			test: "add('12Ki', `13`)",
			err:  true,
		},
		// Duration
		{
			name:           "Duration + Duration -> Duration",
			test:           "add('12s', '13s')",
			expectedResult: `25s`,
		},
		{
			name: "Duration + Scalar -> error",
			test: "add('12s', `13`)",
			err:  true,
		},
		{
			name: "Duration + Quantity -> error",
			test: "add('12s', '13Ki')",
			err:  true,
		},
		{
			name: "Duration + Quantity -> error",
			test: "add('12s', '13')",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Sum(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Scalar
		{
			name: "sum([]) -> error",
			test: "sum([])",
			err:  true,
		},
		{
			name:           "sum(Scalar[]) -> Scalar",
			test:           "sum([`12`])",
			expectedResult: 12.0,
			retFloat:       true,
		},
		{
			name:           "sum(Scalar[]) -> Scalar",
			test:           "sum([`12`, `13`, `1`, `4`])",
			expectedResult: 30.0,
			retFloat:       true,
		},
		{
			name:           "sum(Scalar[]) -> Scalar",
			test:           "sum([`12`, `13`])",
			expectedResult: 25.0,
			retFloat:       true,
		},
		{
			name: "sum(Scalar[Scalar, Duration, ..]) -> error",
			test: "sum(['12', '13s'])",
			err:  true,
		},
		{
			name: "sum(Scalar[Scalar, Quantity, ..]) -> error",
			test: "sum([`12`, '13Ki'])",
			err:  true,
		},
		{
			name: "sum(Scalar[Scalar, Quatity, ..]) -> error",
			test: "sum([`12`, '13'])",
			err:  true,
		},
		// Quantity
		{
			name: "sum([]) -> error",
			test: "sum([])",
			err:  true,
		},
		{
			name:           "sum(Quantity[]) -> Quantity",
			test:           "sum(['12Ki'])",
			expectedResult: `12Ki`,
		},
		{
			name:           "sum(Quantity[]) -> Quantity",
			test:           "sum(['12Ki', '13Ki', '1Ki', '4Ki'])",
			expectedResult: `30Ki`,
		},
		{
			name:           "sum(Quantity[]) -> Quantity",
			test:           "sum(['12Ki', '13Ki'])",
			expectedResult: `25Ki`,
		},
		{
			name:           "sum(Quantity[]) -> Quantity",
			test:           "sum(['12Ki', '13'])",
			expectedResult: `12301`,
		},
		{
			name: "sum(Quantity[Quantity, Duration, ..]) -> error",
			test: "sum(['12Ki', '13s'])",
			err:  true,
		},
		{
			name: "sum(Quantity[Quantity, Scalar, ..]) -> error",
			test: "sum(['12Ki', `13`])",
			err:  true,
		},
		// Duration
		{
			name: "sum([]) -> error",
			test: "sum([])",
			err:  true,
		},
		{
			name:           "sum(Duration[]) -> Duration",
			test:           "sum(['12s'])",
			expectedResult: `12s`,
		},
		{
			name:           "sum(Duration[]) -> Duration",
			test:           "sum(['12s', '13s', '1s', '4s'])",
			expectedResult: `30s`,
		},
		{
			name:           "sum(Duration[]) -> Duration",
			test:           "sum(['12s', '13s'])",
			expectedResult: `25s`,
		},
		{
			name: "sum(Duration[Duration, Scalar, ..]) -> error",
			test: "sum(['12s', `13`])",
			err:  true,
		},
		{
			name: "sum(Duration[Duration, Quantity, ..]) -> error",
			test: "sum(['12s', '13Ki'])",
			err:  true,
		},
		{
			name: "sum(Duration[Duration, Quantity, ..]) -> error",
			test: "sum(['12s', '13'])",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Subtract(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Scalar
		{
			name:           "Scalar - Scalar -> Scalar",
			test:           "subtract(`12`, `13`)",
			expectedResult: -1.0,
			retFloat:       true,
		},
		{
			name: "Scalar - Duration -> error",
			test: "subtract('12', '13s')",
			err:  true,
		},
		{
			name: "Scalar - Quantity -> error",
			test: "subtract(`12`, '13Ki')",
			err:  true,
		},
		{
			name: "Scalar - Quantity -> error",
			test: "subtract(`12`, '13')",
			err:  true,
		},
		// Quantity
		{
			name:           "Quantity - Quantity -> Quantity",
			test:           "subtract('12Ki', '13Ki')",
			expectedResult: `-1Ki`,
		},
		{
			name:           "Quantity - Quantity -> Quantity",
			test:           "subtract('12Ki', '13')",
			expectedResult: `12275`,
		},
		{
			name: "Quantity - Duration -> error",
			test: "subtract('12Ki', '13s')",
			err:  true,
		},
		{
			name: "Quantity - Scalar -> error",
			test: "subtract('12Ki', `13`)",
			err:  true,
		},
		// Duration
		{
			name:           "Duration - Duration -> Duration",
			test:           "subtract('12s', '13s')",
			expectedResult: `-1s`,
		},
		{
			name: "Duration - Scalar -> error",
			test: "subtract('12s', `13`)",
			err:  true,
		},
		{
			name: "Duration - Quantity -> error",
			test: "subtract('12s', '13Ki')",
			err:  true,
		},
		{
			name: "Duration - Quantity -> error",
			test: "subtract('12s', '13')",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Multiply(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Quantity
		{
			name:           "Quantity * Scalar -> Quantity",
			test:           "multiply('12Ki', `2`)",
			expectedResult: `24Ki`,
		},
		{
			name: "Quantity * Quantity -> error",
			test: "multiply('12Ki', '12Ki')",
			err:  true,
		},
		{
			name: "Quantity * Quantity -> error",
			test: "multiply('12Ki', '12')",
			err:  true,
		},
		{
			name: "Quantity * Duration -> error",
			test: "multiply('12Ki', '12s')",
			err:  true,
		},
		// Duration
		{
			name:           "Duration * Scalar -> Duration",
			test:           "multiply('12s', `2`)",
			expectedResult: `24s`,
		},
		{
			name: "Duration * Quantity -> error",
			test: "multiply('12s', '12Ki')",
			err:  true,
		},
		{
			name: "Duration * Quantity -> error",
			test: "multiply('12s', '12')",
			err:  true,
		},
		{
			name: "Duration * Duration -> error",
			test: "multiply('12s', '12s')",
			err:  true,
		},
		// Scalar
		{
			name:           "Scalar * Scalar -> Scalar",
			test:           "multiply(`2.5`, `2.5`)",
			expectedResult: 2.5 * 2.5,
			retFloat:       true,
		},
		{
			name:           "Scalar * Quantity -> Quantity",
			test:           "multiply(`2.5`, '12Ki')",
			expectedResult: "30Ki",
		},
		{
			name:           "Scalar * Quantity -> Quantity",
			test:           "multiply(`2.5`, '12')",
			expectedResult: "30",
		},
		{
			name:           "Scalar * Duration -> Duration",
			test:           "multiply(`2.5`, '40s')",
			expectedResult: "1m40s",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Divide(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Quantity
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('256M', '256M')",
			expectedResult: 1.0,
			retFloat:       true,
		},
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('512M', '256M')",
			expectedResult: 2.0,
			retFloat:       true,
		},
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('8', '3')",
			expectedResult: 8.0 / 3.0,
			retFloat:       true,
		},
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('128M', '256M')",
			expectedResult: 0.5,
			retFloat:       true,
		},
		{
			name:           "Quantity / Scalar -> Quantity",
			test:           "divide('12Ki', `3`)",
			expectedResult: "4Ki",
		},
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('12Ki', '2Ki')",
			expectedResult: 6.0,
			retFloat:       true,
		},
		{
			name:           "Quantity / Quantity -> Scalar",
			test:           "divide('12Ki', '200')",
			expectedResult: 61.44,
			retFloat:       true,
		},
		{
			name: "Quantity / Duration -> error",
			test: "divide('12Ki', '2s')",
			err:  true,
		},
		// Duration
		{
			name:           "Duration / Scalar -> Duration",
			test:           "divide('12s', `3`)",
			expectedResult: "4s",
		},
		{
			name:           "Duration / Duration -> Scalar",
			test:           "divide('12s', '5s')",
			expectedResult: 2.4,
			retFloat:       true,
		},
		{
			name: "Duration / Quantity -> error",
			test: "divide('12s', '4Ki')",
			err:  true,
		},
		{
			name: "Duration / Quantity -> error",
			test: "divide('12s', '4')",
			err:  true,
		},
		// Scalar
		{
			name:           "Scalar / Scalar -> Scalar",
			test:           "divide(`14`, `3`)",
			expectedResult: 4.666666666666667,
			retFloat:       true,
		},
		{
			name: "Scalar / Duration -> error",
			test: "divide(`14`, '5s')",
			err:  true,
		},
		{
			name: "Scalar / Quantity -> error",
			test: "divide(`14`, '5Ki')",
			err:  true,
		},
		{
			name: "Scalar / Quantity -> error",
			test: "divide(`14`, '5')",
			err:  true,
		},
		// Divide by 0
		{
			name: "Scalar / Zero -> error",
			test: "divide(`14`, `0`)",
			err:  true,
		},
		{
			name: "Quantity / Zero -> error",
			test: "divide('4Ki', `0`)",
			err:  true,
		},
		{
			name: "Quantity / Zero -> error",
			test: "divide('4Ki', '0Ki')",
			err:  true,
		},
		{
			name: "Quantity / Zero -> error",
			test: "divide('4', `0`)",
			err:  true,
		},
		{
			name: "Quantity / Zero -> error",
			test: "divide('4', '0')",
			err:  true,
		},
		{
			name: "Duration / Zero -> error",
			test: "divide('4s', `0`)",
			err:  true,
		},
		{
			name: "Duration / Zero -> error",
			test: "divide('4s', '0s')",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Modulo(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Quantity
		{
			name: "Quantity % Duration -> error",
			test: "modulo('12', '13s')",
			err:  true,
		},
		{
			name: "Quantity % Duration -> error",
			test: "modulo('12Ki', '13s')",
			err:  true,
		},
		{
			name: "Quantity % Scalar -> error",
			test: "modulo('12Ki', `13`)",
			err:  true,
		},
		{
			name:           "Quantity % Quantity -> Quantity",
			test:           "modulo('12Ki', '5Ki')",
			expectedResult: `2Ki`,
		},
		// Duration
		{
			name: "Duration % Quantity -> error",
			test: "modulo('13s', '12')",
			err:  true,
		},
		{
			name: "Duration % Quantity -> error",
			test: "modulo('13s', '12Ki')",
			err:  true,
		},
		{
			name:           "Duration % Duration -> Duration",
			test:           "modulo('13s', '2s')",
			expectedResult: `1s`,
		},
		{
			name: "Duration % Scalar -> error",
			test: "modulo('13s', `2`)",
			err:  true,
		},
		// Scalar
		{
			name: "Scalar % Quantity -> error",
			test: "modulo(`13`, '12')",
			err:  true,
		},
		{
			name: "Scalar % Quantity -> error",
			test: "modulo(`13`, '12Ki')",
			err:  true,
		},
		{
			name: "Scalar % Duration -> error",
			test: "modulo(`13`, '5s')",
			err:  true,
		},
		{
			name:           "Scalar % Scalar -> Scalar",
			test:           "modulo(`13`, `5`)",
			expectedResult: 3.0,
			retFloat:       true,
		},
		// Modulo by 0
		{
			name: "Scalar % Zero -> error",
			test: "modulo(`14`, `0`)",
			err:  true,
		},
		{
			name: "Quantity % Zero -> error",
			test: "modulo('4Ki', `0`)",
			err:  true,
		},
		{
			name: "Quantity % Zero -> error",
			test: "modulo('4Ki', '0Ki')",
			err:  true,
		},
		{
			name: "Quantity % Zero -> error",
			test: "modulo('4', `0`)",
			err:  true,
		},
		{
			name: "Quantity % Zero -> error",
			test: "modulo('4', '0')",
			err:  true,
		},
		{
			name: "Duration % Zero -> error",
			test: "modulo('4s', `0`)",
			err:  true,
		},
		{
			name: "Duration % Zero -> error",
			test: "modulo('4s', '0s')",
			err:  true,
		},
		// Modulo with non int values
		{
			name: "Quantity % Non int -> error",
			test: "modulo('4', '1.5')",
			err:  true,
		},
		{
			name: "Non int % Quantity -> error",
			test: "modulo('4.5', '1')",
			err:  true,
		},
		{
			name: "Scalar % Non int -> error",
			test: "modulo(`14`, `1.5`)",
			err:  true,
		},
		{
			name: "Non int % Scalar -> error",
			test: "modulo(`14.5`, `2`)",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func Test_Round(t *testing.T) {
	testCases := []struct {
		name           string
		test           string
		expectedResult interface{}
		err            bool
		retFloat       bool
	}{
		// Scalar
		{
			name: "Scalar roundoff Quantity -> error",
			test: "round(`23`, '12Ki')",
			err:  true,
		},
		{
			name: "Scalar roundoff Duration -> error",
			test: "round(`21`, '5s')",
			err:  true,
		},
		{
			name:           "Scalar roundoff Scalar -> Scalar",
			test:           "round(`9.414675`, `2`)",
			expectedResult: 9.41,
			retFloat:       true,
		},
		{
			name:           "Scalar roundoff zero -> error",
			test:           "round(`14.123`, `6`)",
			expectedResult: 14.123,
			retFloat:       true,
		},
		// round with non int values
		{
			name: "Scalar roundoff Non int -> error",
			test: "round(`14`, `1.5`)",
			err:  true,
		},
		{
			name: "Scalar roundoff negative int -> error",
			test: "round(`14`, `-2`)",
			err:  true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jp, err := newJMESPath(cfg, tc.test)
			assert.NilError(t, err)

			result, err := jp.Search("")
			if !tc.err {
				assert.NilError(t, err)
			} else {
				assert.Assert(t, err != nil)
				return
			}

			if tc.retFloat {
				equal, ok := result.(float64)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(float64))
			} else {
				equal, ok := result.(string)
				assert.Assert(t, ok)
				assert.Equal(t, equal, tc.expectedResult.(string))
			}
		})
	}
}

func TestScalar_Multiply(t *testing.T) {
	type fields struct {
		float64 float64
	}
	type args struct {
		op2 interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
	}{{
		fields: fields{
			float64: 123,
		},
		args: args{
			op2: true,
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op1 := scalar{
				float64: tt.fields.float64,
			}
			got, err := op1.Multiply(tt.args.op2)
			if (err != nil) != tt.wantErr {
				t.Errorf("Scalar.Multiply() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Scalar.Multiply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseArithemticOperands(t *testing.T) {
	type args struct {
		arguments []interface{}
		operator  string
	}
	tests := []struct {
		name    string
		args    args
		want    operand
		want1   operand
		wantErr bool
	}{{
		args: args{
			arguments: []interface{}{
				true,
				1.0,
			},
		},
		wantErr: true,
	}, {
		args: args{
			arguments: []interface{}{
				1.0,
				true,
			},
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := parseArithemticOperands(tt.args.arguments, tt.args.operator)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArithemticOperands() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseArithemticOperands() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParseArithemticOperands() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
