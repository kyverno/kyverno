package jmespath

import (
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

func (op1 Quantity) Add(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		op1.Quantity.Add(v.Quantity)
		return op1.String(), nil
	case Scalar:
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		op1.Quantity.Add(q)
		return op1.String(), nil
	}

	return nil, nil
}

func (op1 Duration) Add(op2 interface{}) (interface{}, error) {
	var sum int64

	switch v := op2.(type) {
	case Duration:
		sum = op1.Nanoseconds() + v.Nanoseconds()
	case Scalar:
		// Converting the duration to nanoseconds for more precision
		sum = op1.Nanoseconds() + int64(v.float64*math.Pow10(9))
	}

	res, err := time.ParseDuration(fmt.Sprintf("%vns", sum))
	if err != nil {
		return nil, err
	}

	return res.String(), nil
}

func (op1 Scalar) Add(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		return op1.float64 + v.float64, nil
	case Quantity:
		return v.Add(op1)
	case Duration:
		return v.Add(op1)
	}

	return nil, nil
}

func (op1 Quantity) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		op1.Quantity.Sub(v.Quantity)
		return op1.String(), nil
	case Scalar:
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		op1.Quantity.Sub(q)
		return op1.String(), nil
	}

	return nil, nil
}

func (op1 Duration) Subtract(op2 interface{}) (interface{}, error) {
	var diff int64

	switch v := op2.(type) {
	case Duration:
		diff = op1.Nanoseconds() - v.Nanoseconds()
	case Scalar:
		// Converting the duration to nanoseconds for more precision
		diff = op1.Nanoseconds() - int64(v.float64*math.Pow10(9))
	}

	res, err := time.ParseDuration(fmt.Sprintf("%vns", diff))
	if err != nil {
		return nil, err
	}

	return res.String(), nil
}

func (op1 Scalar) Subtract(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Scalar:
		return op1.float64 - v.float64, nil
	case Quantity:
		v.Neg()
		return v.Add(op1)
	case Duration:
		// Converting the duration to nanoseconds for more precision
		diff := int64(op1.float64*math.Pow10(9)) - v.Nanoseconds()
		res, err := time.ParseDuration(fmt.Sprintf("%vns", diff))
		if err != nil {
			return nil, err
		}

		return res.String(), nil
	}

	return nil, nil
}

func (op1 Quantity) Multiply(op2 interface{}) (interface{}, error) {
	switch v := op2.(type) {
	case Quantity:
		var prod inf.Dec
		prod.Mul(op1.Quantity.AsDec(), v.Quantity.AsDec())
		return resource.NewDecimalQuantity(prod, v.Quantity.Format).String(), nil
	case Scalar:
		q, err := resource.ParseQuantity(fmt.Sprintf("%v", v.float64))
		if err != nil {
			return nil, err
		}
		var prod inf.Dec
		prod.Mul(op1.Quantity.AsDec(), q.AsDec())
		return resource.NewDecimalQuantity(prod, op1.Quantity.Format).String(), nil
	}

	return nil, nil
}

func (op1 Duration) Multiply(op2 interface{}) (interface{}, error) {
	var prod float64

	switch v := op2.(type) {
	case Duration:
		prod = op1.Seconds() * v.Seconds()
	case Scalar:
		prod = op1.Seconds() * v.float64
	}

	res, err := time.ParseDuration(fmt.Sprintf("%.9fs", prod))
	if err != nil {
		return nil, err
	}

	return res.String(), nil
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
