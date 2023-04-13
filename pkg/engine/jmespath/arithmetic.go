package jmespath

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

type operand interface {
	Add(interface{}, string) (interface{}, error)
	Subtract(interface{}) (interface{}, error)
	Multiply(interface{}) (interface{}, error)
	Divide(interface{}) (interface{}, error)
	Modulo(interface{}) (interface{}, error)
}

type quantity struct {
	resource.Quantity
}

type duration struct {
	time.Duration
}

type scalar struct {
	float64
}

func parseArithemticOperands(arguments []interface{}, operator string) (operand, operand, error) {
	op := [2]operand{nil, nil}
	for i := 0; i < 2; i++ {
		if tmp, err := validateArg(divide, arguments, i, reflect.Float64); err == nil {
			var sc scalar
			sc.float64 = tmp.Float()
			op[i] = sc
		} else if tmp, err = validateArg(divide, arguments, i, reflect.String); err == nil {
			if q, err := resource.ParseQuantity(tmp.String()); err == nil {
				op[i] = quantity{Quantity: q}
			} else if d, err := time.ParseDuration(tmp.String()); err == nil {
				op[i] = duration{Duration: d}
			}
		}
	}
	if op[0] == nil || op[1] == nil {
		return nil, nil, formatError(genericError, operator, "invalid operands")
	}
	return op[0], op[1], nil
}

// Quantity +|- Quantity          -> Quantity
// Quantity +|- Duration|Scalar   -> error
// Duration +|- Duration          -> Duration
// Duration +|- Quantity|Scalar   -> error
// Scalar   +|- Scalar            -> Scalar
// Scalar   +|- Quantity|Duration -> error

func (op1 quantity) Add(op2 interface{}, operator string) (interface{}, error) {
	switch v := op2.(type) {
	case quantity:
		op1.Quantity.Add(v.Quantity)
		return op1.String(), nil
	default:
		return nil, formatError(typeMismatchError, operator)
	}
}

func (op1 duration) Add(op2 interface{}, operator string) (interface{}, error) {
	switch v := op2.(type) {
	case duration:
		return (op1.Duration + v.Duration).String(), nil
	default:
		return nil, formatError(typeMismatchError, operator)
	}
}

func (op1 scalar) Add(op2 interface{}, operator string) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		return op1.float64 + v.float64, nil
	default:
		return nil, formatError(typeMismatchError, operator)
	}
}

func (op1 quantity) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case quantity:
		op1.Quantity.Sub(v.Quantity)
		return op1.String(), nil
	default:
		return nil, formatError(typeMismatchError, subtract)
	}
}

func (op1 duration) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case duration:
		return (op1.Duration - v.Duration).String(), nil
	default:
		return nil, formatError(typeMismatchError, subtract)
	}
}

func (op1 scalar) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		return op1.float64 - v.float64, nil
	default:
		return nil, formatError(typeMismatchError, subtract)
	}
}

// Quantity * Quantity|Duration	-> error
// Quantity * Scalar   			-> Quantity

// Duration * Quantity|Duration	-> error
// Duration * Scalar   			-> Duration

// Scalar   * Scalar            -> Scalar
// Scalar   * Quantity			-> Quantity
// Scalar   * Duration			-> Duration

func (op1 quantity) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		var prod inf.Dec
		prod.Mul(op1.Quantity.AsDec(), q.AsDec())
		return resource.NewDecimalQuantity(prod, op1.Quantity.Format).String(), nil
	default:
		return nil, formatError(typeMismatchError, multiply)
	}
}

func (op1 duration) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		seconds := op1.Seconds() * v.float64
		return time.Duration(seconds * float64(time.Second)).String(), nil
	default:
		return nil, formatError(typeMismatchError, multiply)
	}
}

func (op1 scalar) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		return op1.float64 * v.float64, nil
	case quantity:
		return v.Multiply(op1)
	case duration:
		return v.Multiply(op1)
	default:
		return nil, formatError(typeMismatchError, multiply)
	}
}

// Quantity / Duration			-> error
// Quantity / Quantity			-> Scalar
// Quantity / Scalar   			-> Quantity

// Duration / Quantity			-> error
// Duration / Duration			-> Scalar
// Duration / Scalar   			-> Duration

// Scalar   / Scalar            -> Scalar
// Scalar   / Quantity			-> error
// Scalar   / Duration			-> error

func (op1 quantity) Divide(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case quantity:
		divisor := v.AsApproximateFloat64()
		if divisor == 0 {
			return nil, formatError(zeroDivisionError, divide)
		}
		dividend := op1.AsApproximateFloat64()
		return dividend / divisor, nil
	case scalar:
		if v.float64 == 0 {
			return nil, formatError(zeroDivisionError, divide)
		}
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		var quo inf.Dec
		scale := inf.Scale(math.Max(float64(op1.AsDec().Scale()), float64(q.AsDec().Scale())))
		quo.QuoRound(op1.AsDec(), q.AsDec(), scale, inf.RoundDown)
		return resource.NewDecimalQuantity(quo, op1.Quantity.Format).String(), nil
	default:
		return nil, formatError(typeMismatchError, divide)
	}
}

func (op1 duration) Divide(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case duration:
		if v.Seconds() == 0 {
			return nil, formatError(zeroDivisionError, divide)
		}
		return op1.Seconds() / v.Seconds(), nil
	case scalar:
		if v.float64 == 0 {
			return nil, formatError(zeroDivisionError, divide)
		}
		seconds := op1.Seconds() / v.float64
		return time.Duration(seconds * float64(time.Second)).String(), nil
	default:
		return nil, formatError(typeMismatchError, divide)
	}
}

func (op1 scalar) Divide(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		if v.float64 == 0 {
			return nil, formatError(zeroDivisionError, divide)
		}
		return op1.float64 / v.float64, nil
	default:
		return nil, formatError(typeMismatchError, divide)
	}
}

// Quantity % Duration|Scalar	-> error
// Quantity % Quantity			-> Quantity

// Duration % Quantity|Scalar	-> error
// Duration % Duration			-> Duration

// Scalar   % Quantity|Duration	-> error
// Scalar   % Scalar            -> Scalar

func (op1 quantity) Modulo(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case quantity:
		f1 := op1.ToDec().AsApproximateFloat64()
		f2 := v.ToDec().AsApproximateFloat64()
		i1 := int64(f1)
		i2 := int64(f2)
		if f1 != float64(i1) {
			return nil, formatError(nonIntModuloError, modulo)
		}
		if f2 != float64(i2) {
			return nil, formatError(nonIntModuloError, modulo)
		}
		if i2 == 0 {
			return nil, formatError(zeroDivisionError, modulo)
		}
		return resource.NewQuantity(i1%i2, op1.Quantity.Format).String(), nil
	default:
		return nil, formatError(typeMismatchError, modulo)
	}
}

func (op1 duration) Modulo(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case duration:
		if v.Duration == 0 {
			return nil, formatError(zeroDivisionError, modulo)
		}
		return (op1.Duration % v.Duration).String(), nil
	default:
		return nil, formatError(typeMismatchError, modulo)
	}
}

func (op1 scalar) Modulo(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case scalar:
		val1 := int64(op1.float64)
		val2 := int64(v.float64)
		if op1.float64 != float64(val1) {
			return nil, formatError(nonIntModuloError, modulo)
		}
		if v.float64 != float64(val2) {
			return nil, formatError(nonIntModuloError, modulo)
		}
		if val2 == 0 {
			return nil, formatError(zeroDivisionError, modulo)
		}
		return float64(val1 % val2), nil
	default:
		return nil, formatError(typeMismatchError, modulo)
	}
}
