/*
 * MinIO Cloud Storage, (C) 2019 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sql

import (
	"errors"
	"fmt"
)

// Aggregation Function name constants
const (
	aggFnAvg   FuncName = "AVG"
	aggFnCount FuncName = "COUNT"
	aggFnMax   FuncName = "MAX"
	aggFnMin   FuncName = "MIN"
	aggFnSum   FuncName = "SUM"
)

var (
	errNonNumericArg = func(fnStr FuncName) error {
		return fmt.Errorf("%s() requires a numeric argument", fnStr)
	}
	errInvalidAggregation = errors.New("Invalid aggregation seen")
)

type aggVal struct {
	runningSum             *Value
	runningCount           int64
	runningMax, runningMin *Value

	// Stores if at least one record has been seen
	seen bool
}

func newAggVal(fn FuncName) *aggVal {
	switch fn {
	case aggFnAvg, aggFnSum:
		return &aggVal{runningSum: FromFloat(0)}
	case aggFnMin:
		return &aggVal{runningMin: FromInt(0)}
	case aggFnMax:
		return &aggVal{runningMax: FromInt(0)}
	default:
		return &aggVal{}
	}
}

// evalAggregationNode - performs partial computation using the
// current row and stores the result.
//
// On success, it returns (nil, nil).
func (e *FuncExpr) evalAggregationNode(r Record) error {
	// It is assumed that this function is called only when
	// `e` is an aggregation function.

	var val *Value
	var err error
	funcName := e.getFunctionName()
	if aggFnCount == funcName {
		if e.Count.StarArg {
			// Handle COUNT(*)
			e.aggregate.runningCount++
			return nil
		}

		val, err = e.Count.ExprArg.evalNode(r)
		if err != nil {
			return err
		}
	} else {
		// Evaluate the (only) argument
		val, err = e.SFunc.ArgsList[0].evalNode(r)
		if err != nil {
			return err
		}
	}

	if val.IsNull() {
		// E.g. the column or field does not exist in the
		// record - in all such cases the aggregation is not
		// updated.
		return nil
	}

	argVal := val
	if funcName != aggFnCount {
		// All aggregation functions, except COUNT require a
		// numeric argument.

		// Here, we diverge from Amazon S3 behavior by
		// inferring untyped values are numbers.
		if !argVal.isNumeric() {
			if i, ok := argVal.bytesToInt(); ok {
				argVal.setInt(i)
			} else if f, ok := argVal.bytesToFloat(); ok {
				argVal.setFloat(f)
			} else {
				return errNonNumericArg(funcName)
			}
		}
	}

	// Mark that we have seen one non-null value.
	isFirstRow := false
	if !e.aggregate.seen {
		e.aggregate.seen = true
		isFirstRow = true
	}

	switch funcName {
	case aggFnCount:
		// For all non-null values, the count is incremented.
		e.aggregate.runningCount++

	case aggFnAvg, aggFnSum:
		e.aggregate.runningCount++
		// Convert to float.
		f, ok := argVal.ToFloat()
		if !ok {
			return fmt.Errorf("Could not convert value %v (%s) to a number", argVal.value, argVal.GetTypeString())
		}
		argVal.setFloat(f)
		err = e.aggregate.runningSum.arithOp(opPlus, argVal)

	case aggFnMin:
		err = e.aggregate.runningMin.minmax(argVal, false, isFirstRow)

	case aggFnMax:
		err = e.aggregate.runningMax.minmax(argVal, true, isFirstRow)

	default:
		err = errInvalidAggregation
	}

	return err
}

func (e *AliasedExpression) aggregateRow(r Record) error {
	return e.Expression.aggregateRow(r)
}

func (e *Expression) aggregateRow(r Record) error {
	for _, ex := range e.And {
		err := ex.aggregateRow(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *ListExpr) aggregateRow(r Record) error {
	for _, ex := range e.Elements {
		err := ex.aggregateRow(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *AndCondition) aggregateRow(r Record) error {
	for _, ex := range e.Condition {
		err := ex.aggregateRow(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Condition) aggregateRow(r Record) error {
	if e.Operand != nil {
		return e.Operand.aggregateRow(r)
	}
	return e.Not.aggregateRow(r)
}

func (e *ConditionOperand) aggregateRow(r Record) error {
	err := e.Operand.aggregateRow(r)
	if err != nil {
		return err
	}

	if e.ConditionRHS == nil {
		return nil
	}

	switch {
	case e.ConditionRHS.Compare != nil:
		return e.ConditionRHS.Compare.Operand.aggregateRow(r)
	case e.ConditionRHS.Between != nil:
		err = e.ConditionRHS.Between.Start.aggregateRow(r)
		if err != nil {
			return err
		}
		return e.ConditionRHS.Between.End.aggregateRow(r)
	case e.ConditionRHS.In != nil:
		elt := e.ConditionRHS.In.ListExpression
		err = elt.aggregateRow(r)
		if err != nil {
			return err
		}
		return nil
	case e.ConditionRHS.Like != nil:
		err = e.ConditionRHS.Like.Pattern.aggregateRow(r)
		if err != nil {
			return err
		}
		return e.ConditionRHS.Like.EscapeChar.aggregateRow(r)
	default:
		return errInvalidASTNode
	}
}

func (e *Operand) aggregateRow(r Record) error {
	err := e.Left.aggregateRow(r)
	if err != nil {
		return err
	}
	for _, rt := range e.Right {
		err = rt.Right.aggregateRow(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *MultOp) aggregateRow(r Record) error {
	err := e.Left.aggregateRow(r)
	if err != nil {
		return err
	}
	for _, rt := range e.Right {
		err = rt.Right.aggregateRow(r)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *UnaryTerm) aggregateRow(r Record) error {
	if e.Negated != nil {
		return e.Negated.Term.aggregateRow(r)
	}
	return e.Primary.aggregateRow(r)
}

func (e *PrimaryTerm) aggregateRow(r Record) error {
	switch {
	case e.ListExpr != nil:
		return e.ListExpr.aggregateRow(r)
	case e.SubExpression != nil:
		return e.SubExpression.aggregateRow(r)
	case e.FuncCall != nil:
		return e.FuncCall.aggregateRow(r)
	}
	return nil
}

func (e *FuncExpr) aggregateRow(r Record) error {
	switch e.getFunctionName() {
	case aggFnAvg, aggFnSum, aggFnMax, aggFnMin, aggFnCount:
		return e.evalAggregationNode(r)
	default:
		// TODO: traverse arguments and call aggregateRow on
		// them if they could be an ancestor of an
		// aggregation.
	}
	return nil
}

// getAggregate() implementation for each AST node follows. This is
// called after calling aggregateRow() on each input row, to calculate
// the final aggregate result.

func (e *FuncExpr) getAggregate() (*Value, error) {
	switch e.getFunctionName() {
	case aggFnCount:
		return FromInt(e.aggregate.runningCount), nil

	case aggFnAvg:
		if e.aggregate.runningCount == 0 {
			// No rows were seen by AVG.
			return FromNull(), nil
		}
		err := e.aggregate.runningSum.arithOp(opDivide, FromInt(e.aggregate.runningCount))
		return e.aggregate.runningSum, err

	case aggFnMin:
		if !e.aggregate.seen {
			// No rows were seen by MIN
			return FromNull(), nil
		}
		return e.aggregate.runningMin, nil

	case aggFnMax:
		if !e.aggregate.seen {
			// No rows were seen by MAX
			return FromNull(), nil
		}
		return e.aggregate.runningMax, nil

	case aggFnSum:
		// TODO: check if returning 0 when no rows were seen
		// by SUM is expected behavior.
		return e.aggregate.runningSum, nil

	default:
		// TODO:
	}

	return nil, errInvalidAggregation
}
