package jmespath

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"time"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Operand interface {
	Add(interface{}) (interface{}, error)
	Subtract(interface{}) (interface{}, error)
	Multiply(interface{}) (interface{}, error)
	Divide(interface{}) (interface{}, error)
	Modulo(interface{}) (interface{}, error)
}

type Quantity struct {
	resource.Quantity
}

type Duration struct {
	time.Duration
}

type Scalar struct {
	float64
}

var errTypeMismatch = errors.New("types mismatch")

func ParseArithemticOperands(arguments []interface{}, operator string) (Operand, Operand, error) {
	op := [2]Operand{nil, nil}
	t := [2]int{0, 0}

	for i := 0; i < 2; i++ {
		tmp, err := validateArg(divide, arguments, i, reflect.Float64)
		if err == nil {
			var sc Scalar
			sc.float64 = tmp.Float()
			op[i] = sc
		}

		tmp, err = validateArg(divide, arguments, i, reflect.String)
		if err == nil {
			var q Quantity
			q.Quantity, err = resource.ParseQuantity(tmp.String())
			if err == nil {
				op[i] = q
				t[i] = 1
			} else {
				var d Duration
				d.Duration, err = time.ParseDuration(tmp.String())
				if err == nil {
					op[i] = d
					t[i] = 2
				}
			}
		}
	}

	if op[0] == nil || op[1] == nil || t[0]|t[1] == 3 {
		return nil, nil, fmt.Errorf(genericError, operator, "invalid operands")
	}

	return op[0], op[1], nil
}

// Quantity +|- Quantity          -> Quantity
// Quantity +|- Duration|Scalar   -> error
// Duration +|- Duration          -> Duration
// Duration +|- Quantity|Scalar   -> error
// Scalar   +|- Scalar            -> Scalar
// Scalar   +|- Quantity|Duration -> error

func (op1 Quantity) Add(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		op1.Quantity.Add(v.Quantity)
		return op1.String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Duration) Add(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Duration:
		return (op1.Duration + v.Duration).String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Scalar) Add(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		return op1.float64 + v.float64, nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Quantity) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		op1.Quantity.Sub(v.Quantity)
		return op1.String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Duration) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Duration:
		return (op1.Duration - v.Duration).String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Scalar) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		return op1.float64 - v.float64, nil
	default:
		return nil, errTypeMismatch
	}
}

// Quantity * Quantity|Duration	-> error
// Quantity * Scalar   			-> Quantity

// Duration * Quantity|Duration	-> error
// Duration * Scalar   			-> Duration

// Scalar   * Scalar            -> Scalar
// Scalar   * Quantity			-> Quantity
// Scalar   * Duration			-> Duration

func (op1 Quantity) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		var prod inf.Dec
		prod.Mul(op1.Quantity.AsDec(), q.AsDec())
		return resource.NewDecimalQuantity(prod, op1.Quantity.Format).String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Duration) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		seconds := op1.Seconds() * v.float64
		return time.Duration(seconds * float64(time.Second)).String(), nil
	default:
		return nil, errTypeMismatch
	}
}

func (op1 Scalar) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		return op1.float64 * v.float64, nil
	case Quantity:
		return v.Multiply(op1)
	case Duration:
		return v.Multiply(op1)
	}

	return nil, nil
}

func (op1 Quantity) Divide(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		if v.ToDec().AsApproximateFloat64() == 0 {
			return nil, fmt.Errorf(zeroDivisionError, divide)
		}
		var quo inf.Dec
		scale := inf.Scale(math.Max(float64(op1.AsDec().Scale()), float64(v.AsDec().Scale())))
		quo.QuoRound(op1.AsDec(), v.AsDec(), scale, inf.RoundDown)
		return strconv.ParseFloat(quo.String(), 64)
	case Scalar:
		if v.float64 == 0 {
			return nil, fmt.Errorf(zeroDivisionError, divide)
		}
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}

		var quo inf.Dec
		scale := inf.Scale(math.Max(float64(op1.AsDec().Scale()), float64(q.AsDec().Scale())))
		quo.QuoRound(op1.AsDec(), q.AsDec(), scale, inf.RoundDown)
		return resource.NewDecimalQuantity(quo, op1.Quantity.Format).String(), nil
	}

	return nil, nil
}

func (op1 Duration) Divide(op2 interface{}) (interface{}, error) {
	var quo float64

	switch v := op2.(type) {
	case Duration:
		if v.Seconds() == 0 {
			return nil, fmt.Errorf(undefinedQuoError, divide)
		}

		quo = op1.Seconds() / v.Seconds()
		return quo, nil
	case Scalar:
		if v.float64 == 0 {
			return nil, fmt.Errorf(undefinedQuoError, divide)
		}

		quo = op1.Seconds() / v.float64
		res, err := time.ParseDuration(fmt.Sprintf("%.9fs", quo))
		if err != nil {
			return nil, err
		}

		return res.String(), nil
	}

	return nil, nil
}

func (op1 Scalar) Divide(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		if v.float64 == 0 {
			return nil, fmt.Errorf(zeroDivisionError, divide)
		}

		return op1.float64 / v.float64, nil
	case Quantity:
		if v.ToDec().AsApproximateFloat64() == 0 {
			return nil, fmt.Errorf(zeroDivisionError, divide)
		}
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", op1.float64))
		if err != nil {
			return nil, err
		}

		var quo inf.Dec
		scale := inf.Scale(math.Max(float64(q.AsDec().Scale()), float64(v.AsDec().Scale())))
		quo.QuoRound(q.AsDec(), v.AsDec(), scale, inf.RoundDown)

		return resource.NewDecimalQuantity(quo, v.Format).String(), nil
	case Duration:
		var quo float64
		if op1.float64 == 0 {
			return nil, fmt.Errorf(undefinedQuoError, divide)
		}

		quo = op1.float64 / v.Seconds()

		res, err := time.ParseDuration(fmt.Sprintf("%.9fs", quo))
		if err != nil {
			return nil, err
		}

		return res.String(), nil
	}

	return nil, nil
}

func (op1 Quantity) Modulo(op2 interface{}) (interface{}, error) {
	quo, err := op1.Divide(op2)
	if err != nil {
		return nil, err
	}

	var x resource.Quantity
	switch y := quo.(type) {
	case float64:
		x, err = resource.ParseQuantity(fmt.Sprintf("%.9f", y))
	case string:
		x, err = resource.ParseQuantity(y)
	}
	if err != nil {
		return nil, err
	}

	mul, err := op2.(Operand).Multiply(Quantity{x})
	if err != nil {
		return nil, err
	}

	y, err := resource.ParseQuantity(mul.(string))
	if err != nil {
		return nil, err
	}

	return op1.Subtract(Quantity{y})
}

func (op1 Duration) Modulo(op2 interface{}) (interface{}, error) {
	quo, err := op1.Divide(op2)
	if err != nil {
		return nil, err
	}

	var x time.Duration
	switch y := quo.(type) {
	case float64:
		x, err = time.ParseDuration(fmt.Sprintf("%.9fs", y))
	case string:
		x, err = time.ParseDuration(y)
	}
	if err != nil {
		return nil, err
	}
	x = x.Truncate(time.Second)

	mul, err := op2.(Operand).Multiply(Duration{x})
	if err != nil {
		return nil, err
	}
	y, err := time.ParseDuration(mul.(string))
	if err != nil {
		return nil, err
	}

	return op1.Subtract(Duration{y})
}

func (op1 Scalar) Modulo(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		val1 := int64(op1.float64)
		val2 := int64(v.float64)

		if op1.float64 != float64(val1) {
			return nil, fmt.Errorf(nonIntModuloError, modulo)
		}

		if v.float64 != float64(val2) {
			return nil, fmt.Errorf(nonIntModuloError, modulo)
		}

		if val2 == 0 {
			return nil, fmt.Errorf(zeroDivisionError, modulo)
		}

		return float64(val1 % val2), nil
	case Quantity:
		quo, err := op1.Divide(op2)
		if err != nil {
			return nil, err
		}

		x, err := resource.ParseQuantity(quo.(string))
		if err != nil {
			return nil, err
		}

		mul, err := op2.(Operand).Multiply(Quantity{x})
		if err != nil {
			return nil, err
		}

		y, err := resource.ParseQuantity(mul.(string))
		if err != nil {
			return nil, err
		}

		return op1.Subtract(Quantity{y})
	case Duration:
		quo, err := op1.Divide(op2)
		if err != nil {
			return nil, err
		}

		x, err := time.ParseDuration(quo.(string))
		if err != nil {
			return nil, err
		}
		x = x.Truncate(time.Second)

		mul, err := op2.(Operand).Multiply(Duration{x})
		if err != nil {
			return nil, err
		}
		y, err := time.ParseDuration(mul.(string))
		if err != nil {
			return nil, err
		}

		return op1.Subtract(Duration{y})
	}

	return nil, nil
}
