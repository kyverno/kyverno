package jmespath

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Operand interface {
	Add(interface{}) (interface{}, error)
	Subtract(interface{}) (interface{}, error)
	Multiply(interface{}) (interface{}, error)
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

func ParseArithemticOperands(arguments []interface{}) (Operand, Operand, error) {
	var op1, op2 Operand = nil, nil
	tmp1, err := ValidateArg(divide, arguments, 0, reflect.Float64)
	if err == nil {
		var sc Scalar
		sc.float64 = tmp1.Float()
		op1 = sc
	}

	tmp1, err = ValidateArg(divide, arguments, 0, reflect.String)
	if err == nil {
		var q Quantity
		q.Quantity, err = resource.ParseQuantity(tmp1.String())
		if err == nil {
			op1 = q
		} else {
			var d Duration
			d.Duration, err = time.ParseDuration(tmp1.String())
			if err == nil {
				op1 = d
			}
		}
	}

	tmp2, err := ValidateArg(divide, arguments, 1, reflect.Float64)
	if err == nil {
		var sc Scalar
		sc.float64 = tmp2.Float()
		op2 = sc
	}

	tmp2, err = ValidateArg(divide, arguments, 1, reflect.String)
	if err == nil {
		var q Quantity
		q.Quantity, err = resource.ParseQuantity(tmp2.String())
		if err == nil {
			op2 = q
		} else {

			var d Duration
			d.Duration, err = time.ParseDuration(tmp2.String())
			if err == nil {
				op2 = d
			}
		}
	}

	if op1 == nil || op2 == nil {
		return nil, nil, errors.New("Invalid operands")
	}

	return op1, op2, nil
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
	var sum float64

	switch v := op2.(type) {
	case Duration:
		sum = op1.Seconds() + v.Seconds()
	case Scalar:
		sum = op1.Seconds() + v.float64
	}

	res, err := time.ParseDuration(fmt.Sprintf("%vs", sum))
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
	var sum float64

	switch v := op2.(type) {
	case Duration:
		sum = op1.Seconds() - v.Seconds()
	case Scalar:
		sum = op1.Seconds() - v.float64
	}

	res, err := time.ParseDuration(fmt.Sprintf("%vs", sum))
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
		sum := op1.float64 - v.Seconds()
		res, err := time.ParseDuration(fmt.Sprintf("%vs", sum))
		if err != nil {
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
		var prod inf.Dec
		prod.Mul(op1.Quantity.AsDec(), q.AsDec())
		if err != nil {
			return nil, err
		}
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

	res, err := time.ParseDuration(fmt.Sprintf("%vs", prod))
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
