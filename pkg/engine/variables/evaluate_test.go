package variables

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// STRINGS
func Test_Eval_Equal_Const_String_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      "name",
		Operator: kyverno.Equal,
		Value:    "name",
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_String_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      "name",
		Operator: kyverno.Equal,
		Value:    "name1",
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NoEqual_Const_String_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      "name",
		Operator: kyverno.NotEqual,
		Value:    "name1",
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NoEqual_Const_String_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      "name",
		Operator: kyverno.NotEqual,
		Value:    "name",
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_string_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.GreaterThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_string_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.GreaterThanOrEquals,
		Value:    0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_string_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1.1",
		Operator: kyverno.GreaterThanOrEquals,
		Value:    "2",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_string_Equal_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.GreaterThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_string_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.GreaterThan,
		Value:    0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThan_Const_string_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1.1",
		Operator: kyverno.GreaterThan,
		Value:    "2",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThanOrEquals_Const_string_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.LessThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_string_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "0",
		Operator: kyverno.LessThanOrEquals,
		Value:    1,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_string_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "2.0",
		Operator: kyverno.LessThanOrEquals,
		Value:    "1.1",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_string_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "1",
		Operator: kyverno.LessThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_string_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "0",
		Operator: kyverno.LessThan,
		Value:    1,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThan_Const_string_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      "2.0",
		Operator: kyverno.LessThan,
		Value:    "1.1",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

//Bool

func Test_Eval_Equal_Const_Bool_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      true,
		Operator: kyverno.Equal,
		Value:    true,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_Bool_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      true,
		Operator: kyverno.Equal,
		Value:    false,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NoEqual_Const_Bool_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      true,
		Operator: kyverno.NotEqual,
		Value:    false,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NoEqual_Const_Bool_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      true,
		Operator: kyverno.NotEqual,
		Value:    true,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// int
func Test_Eval_Equal_Const_int_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.Equal,
		Value:    1,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.Equal,
		Value:    2,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NoEqual_Const_int_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.NotEqual,
		Value:    2,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NoEqual_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.NotEqual,
		Value:    1,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_int_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_int_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    "2",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_int_Equal_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_int_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThan,
		Value:    0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThan_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.GreaterThan,
		Value:    "2",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThanOrEquals_Const_int_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.LessThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_int_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      0,
		Operator: kyverno.LessThanOrEquals,
		Value:    1,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      2,
		Operator: kyverno.LessThanOrEquals,
		Value:    "1",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_int_Equal_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1,
		Operator: kyverno.LessThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_int_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      0,
		Operator: kyverno.LessThan,
		Value:    1,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThan_Const_int_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      2,
		Operator: kyverno.LessThan,
		Value:    "1",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// int64
func Test_Eval_Equal_Const_int64_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      int64(1),
		Operator: kyverno.Equal,
		Value:    int64(1),
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_int64_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      int64(1),
		Operator: kyverno.Equal,
		Value:    int64(2),
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NoEqual_Const_int64_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      int64(1),
		Operator: kyverno.NotEqual,
		Value:    int64(2),
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NoEqual_Const_int64_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      int64(1),
		Operator: kyverno.NotEqual,
		Value:    int64(1),
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

//float64

func Test_Eval_Equal_Const_float64_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.Equal,
		Value:    1.5,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.Equal,
		Value:    1.6,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NoEqual_Const_float64_Pass(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.NotEqual,
		Value:    1.6,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NoEqual_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	// no variables
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.NotEqual,
		Value:    1.5,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_float64_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.0,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_float64_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThanOrEquals_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.95,
		Operator: kyverno.GreaterThanOrEquals,
		Value:    "2",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_float64_Equal_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.0,
		Operator: kyverno.GreaterThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_GreaterThan_Const_float64_Greater_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.5,
		Operator: kyverno.GreaterThan,
		Value:    "0",
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_GreaterThan_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.95,
		Operator: kyverno.GreaterThan,
		Value:    "2.5",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThanOrEquals_Const_float64_Equal_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.0,
		Operator: kyverno.LessThanOrEquals,
		Value:    1.0,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_float64_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      0.5,
		Operator: kyverno.LessThanOrEquals,
		Value:    1,
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThanOrEquals_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      2.0,
		Operator: kyverno.LessThanOrEquals,
		Value:    "1.95",
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_float64_Equal_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      1.0,
		Operator: kyverno.LessThan,
		Value:    1.0,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_LessThan_Const_float64_Less_Pass(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      0.5,
		Operator: kyverno.LessThan,
		Value:    "1.5",
	}
	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_LessThan_Const_float64_Fail(t *testing.T) {
	ctx := context.NewContext()
	condition := kyverno.Condition{
		Key:      2.5,
		Operator: kyverno.LessThan,
		Value:    1.95,
	}
	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

//object/map[string]interface

func Test_Eval_Equal_Const_object_Pass(t *testing.T) {
	ctx := context.NewContext()
	var err error

	obj1Raw := []byte(`{ "dir": { "file1": "a" } }`)
	obj2Raw := []byte(`{ "dir": { "file1": "a" } }`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}

	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.Equal,
		Value:    obj2,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_object_Fail(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`{ "dir": { "file1": "a" } }`)
	obj2Raw := []byte(`{ "dir": { "file1": "b" } }`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}

	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}

	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.Equal,
		Value:    obj2,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NotEqual_Const_object_Pass(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`{ "dir": { "file1": "a" } }`)
	obj2Raw := []byte(`{ "dir": { "file1": "b" } }`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}

	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}

	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.NotEqual,
		Value:    obj2,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NotEqual_Const_object_Fail(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`{ "dir": { "file1": "a" } }`)
	obj2Raw := []byte(`{ "dir": { "file1": "a" } }`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}

	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}

	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.NotEqual,
		Value:    obj2,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// list/ []interface{}

func Test_Eval_Equal_Const_list_Pass(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	obj2Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}

	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}

	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.Equal,
		Value:    obj2,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Const_list_Fail(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	obj2Raw := []byte(`[ { "name": "b", "file": "a" }, { "name": "b", "file": "b" } ]`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}
	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.Equal,
		Value:    obj2,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

func Test_Eval_NotEqual_Const_list_Pass(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	obj2Raw := []byte(`[ { "name": "b", "file": "a" }, { "name": "b", "file": "b" } ]`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}
	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.NotEqual,
		Value:    obj2,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_NotEqual_Const_list_Fail(t *testing.T) {
	ctx := context.NewContext()
	var err error
	obj1Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	obj2Raw := []byte(`[ { "name": "a", "file": "a" }, { "name": "b", "file": "b" } ]`)
	var obj1, obj2 interface{}
	err = json.Unmarshal(obj1Raw, &obj1)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(obj2Raw, &obj2)
	if err != nil {
		t.Error(err)
	}
	// no variables
	condition := kyverno.Condition{
		Key:      obj1,
		Operator: kyverno.NotEqual,
		Value:    obj2,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// Variables

func Test_Eval_Equal_Var_Pass(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	// context
	ctx := context.NewContext()
	err := ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}
	condition := kyverno.Condition{
		Key:      "{{request.object.metadata.name}}",
		Operator: kyverno.Equal,
		Value:    "temp",
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

func Test_Eval_Equal_Var_Fail(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	// context
	ctx := context.NewContext()
	err := ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}
	condition := kyverno.Condition{
		Key:      "{{request.object.metadata.name}}",
		Operator: kyverno.Equal,
		Value:    "temp1",
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// subset test

// test passes if ALL values in "key" are in "value" ("key" is a subset of "value")
func Test_Eval_In_String_Set_Pass(t *testing.T) {
	ctx := context.NewContext()
	key := [2]string{"1.1.1.1", "2.2.2.2"}
	keyInterface := make([]interface{}, len(key), len(key))
	for i := range key {
		keyInterface[i] = key[i]
	}
	value := [3]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	valueInterface := make([]interface{}, len(value), len(value))
	for i := range value {
		valueInterface[i] = value[i]
	}

	condition := kyverno.Condition{
		Key:      keyInterface,
		Operator: kyverno.In,
		Value:    valueInterface,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

// test passes if NOT ALL values in "key" are in "value" ("key" is not a subset of "value")
func Test_Eval_In_String_Set_Fail(t *testing.T) {
	ctx := context.NewContext()
	key := [2]string{"1.1.1.1", "4.4.4.4"}
	keyInterface := make([]interface{}, len(key), len(key))
	for i := range key {
		keyInterface[i] = key[i]
	}
	value := [3]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	valueInterface := make([]interface{}, len(value), len(value))
	for i := range value {
		valueInterface[i] = value[i]
	}

	condition := kyverno.Condition{
		Key:      keyInterface,
		Operator: kyverno.In,
		Value:    valueInterface,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}

// test passes if ALL of the values in "key" are NOT in "value" ("key" is not a subset of "value")
func Test_Eval_NotIn_String_Set_Pass(t *testing.T) {
	ctx := context.NewContext()
	key := [2]string{"1.1.1.1", "4.4.4.4"}
	keyInterface := make([]interface{}, len(key), len(key))
	for i := range key {
		keyInterface[i] = key[i]
	}
	value := [3]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	valueInterface := make([]interface{}, len(value), len(value))
	for i := range value {
		valueInterface[i] = value[i]
	}

	condition := kyverno.Condition{
		Key:      keyInterface,
		Operator: kyverno.NotIn,
		Value:    valueInterface,
	}

	if !Evaluate(log.Log, ctx, condition) {
		t.Error("expected to pass")
	}
}

// test passes if ALL of the values in "key" are in "value" ("key" is a subset of "value")
func Test_Eval_NotIn_String_Set_Fail(t *testing.T) {
	ctx := context.NewContext()
	key := [2]string{"1.1.1.1", "2.2.2.2"}
	keyInterface := make([]interface{}, len(key), len(key))
	for i := range key {
		keyInterface[i] = key[i]
	}
	value := [3]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	valueInterface := make([]interface{}, len(value), len(value))
	for i := range value {
		valueInterface[i] = value[i]
	}

	condition := kyverno.Condition{
		Key:      keyInterface,
		Operator: kyverno.NotIn,
		Value:    valueInterface,
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}
