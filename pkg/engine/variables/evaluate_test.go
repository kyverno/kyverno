package variables

import (
	"testing"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
)

func TestEvaluate(t *testing.T) {
	testCases := []struct {
		Condition kyverno.Condition
		Result    bool
	}{
		// Equals
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("string")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(1.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(true)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(false)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1024"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1024")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1024Mi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("60m")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(map[string]interface{}{"foo": "bar"}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(map[string]interface{}{"foo": "bar"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"foo", "bar"}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON([]interface{}{"foo", "bar"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("not string")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(2)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(int64(2))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(2)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(2.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(false)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(true)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("10Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1024Mi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("5h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("30m")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("2")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("2.0")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("false")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("true")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(map[string]interface{}{"foo": "bar"}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(map[string]interface{}{"bar": "foo"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"foo", "bar"}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON([]interface{}{"bar", "foo"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}}), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON([]interface{}{map[string]string{"bar": "foo"}})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("2h"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON(3600)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.2"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1.5.2")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.2"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1.5.*")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["Equals"], RawValue: kyverno.ToJSON("1.5.5")}, false},

		// Not Equals
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("string")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(1.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(false)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(false)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1024"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1Ki")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1024")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1023"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1023")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1024Mi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("60m")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(map[string]interface{}{"foo": "bar"}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(map[string]interface{}{"foo": "bar"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"foo", "bar"}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON([]interface{}{"foo", "bar"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("not string")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(2)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(int64(2))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(2)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(2.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(true)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(true)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("10Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1024Mi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("5h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("30m")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("string"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("2")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("2.0")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(true), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("false")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(false), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("true")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(map[string]interface{}{"foo": "bar"}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(map[string]interface{}{"bar": "foo"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"foo", "bar"}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON([]interface{}{"bar", "foo"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{map[string]string{"foo": "bar"}}), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON([]interface{}{map[string]string{"bar": "foo"}})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(3600)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("2h"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.2"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1.5.5")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.2"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1.5.*")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["NotEquals"], RawValue: kyverno.ToJSON("1.5.0")}, false},

		// Greater Than
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1.5)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1.5)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1025"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1023")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1Mi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Mi"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10Mi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10h"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("30m")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(100), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("100"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(3600)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("2h"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(3600), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(3600), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("30m")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(10))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(int64(10))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(-5), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(-5), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(-10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON(-10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1.5.0")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["GreaterThan"], RawValue: kyverno.ToJSON("1.5.5")}, false},

		// Less Than
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1.5)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1.5)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1023"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1025")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1Mi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Mi"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10h"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("30m")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(100), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("100"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(3600)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("30m"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(3600), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(3600), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("30m")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(10))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(int64(10))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(-5), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(-5), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(-10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON(-10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1.5.0")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["LessThan"], RawValue: kyverno.ToJSON("1.5.5")}, true},

		// Greater Than or Equal
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1.5)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1.5)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1025"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1024"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1023")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1024")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1Mi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10h"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("30m")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(100), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("100"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("2h"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(10))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(10)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON(int64(10))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1.5.5")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1.5.0")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], RawValue: kyverno.ToJSON("1.5.5")}, false},

		// Less Than or Equal
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1.0)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1.5)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1.5)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.0), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1.0)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1024")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1024"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Ki"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1025")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1023"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Ki")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10Gi"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Gi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Mi")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Mi"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10h"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1h")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("30m")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1h")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1Gi"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1Gi")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(100), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("100"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("10"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1h"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(3600)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("2h"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(3600)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(10))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(10)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(1)}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(int64(1)), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(10)}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(10), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(1))}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON(int64(10))}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1.5.5")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.0"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1.5.5")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.5.5"), Operator: kyverno.ConditionOperators["LessThanOrEquals"], RawValue: kyverno.ToJSON("1.5.0")}, false},

		// In
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2"}), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5.5), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.1.1.1"), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{"1*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["In"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},

		// Not In
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2"}), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5.5), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["NotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},

		// Any In
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("1.1.1.1")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"4.4.4.4", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 2}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{5}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1*"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5*"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"2*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"4*"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5"}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("1-3")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1.1.1.1"), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON([]interface{}{"1*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("1-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("1-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("7-10")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("0-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1.002, 1.222}), Operator: kyverno.ConditionOperators["AnyIn"], RawValue: kyverno.ToJSON("1.001-10")}, true},

		// All In
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"4.4.4.4", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 2}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{5}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3, 4})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1*"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5*"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"2.1.1.1", "2.2.2.2", "2.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"2*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON([]interface{}{"4*"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON("5.5.5.5")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON("1-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{3, 2}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON("1-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{3, 2}), Operator: kyverno.ConditionOperators["AllIn"], RawValue: kyverno.ToJSON("5-10")}, false},

		// All Not In
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5.5), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4", "1.1.1.1"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1", "4.4.4.4"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5.5.5.5", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"7*", "6*", "5*"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1*", "2*"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "3.3.3.3", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"2*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"4.1.1.1", "4.2.2.2", "4.5.5.5"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON([]interface{}{"4*"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON("2.2.2.2")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5.5), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON("6-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON("1-6")}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{3, 2}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON("5-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{2, 6}), Operator: kyverno.ConditionOperators["AllNotIn"], RawValue: kyverno.ToJSON("5-10")}, false},

		// Any Not In
		{kyverno.Condition{RawKey: kyverno.ToJSON(1), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(1.5), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("1"), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON(5.5), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{1, 1.5, 2, 3})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1", "2", "3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5.5.5.5", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON("4.4.4.4")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"5.5.5.5", "4.4.4.4"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1*", "3*", "5*"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"2*"})}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{"2.2*"}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON([]interface{}{"2.2.2.2"})}, false},
		{kyverno.Condition{RawKey: kyverno.ToJSON("5"), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON("1-3")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5, 11}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON("0-10")}, true},
		{kyverno.Condition{RawKey: kyverno.ToJSON([]interface{}{1, 5, 7}), Operator: kyverno.ConditionOperators["AnyNotIn"], RawValue: kyverno.ToJSON("0-10")}, false},
	}

	ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
	for _, tc := range testCases {
		if val, _, _ := Evaluate(logr.Discard(), ctx, tc.Condition); val != tc.Result {
			t.Errorf("%v - expected result to be %v", tc.Condition, tc.Result)
		}
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
	ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
	err := context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}
	condition := kyverno.Condition{
		RawKey:   kyverno.ToJSON("{{request.object.metadata.name}}"),
		Operator: kyverno.ConditionOperators["Equal"],
		RawValue: kyverno.ToJSON("temp"),
	}

	val, _, _ := Evaluate(logr.Discard(), ctx, condition)
	assert.True(t, val)
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
	ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
	err := context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}
	condition := kyverno.Condition{
		RawKey:   kyverno.ToJSON("{{request.object.metadata.name}}"),
		Operator: kyverno.ConditionOperators["Equal"],
		RawValue: kyverno.ToJSON("temp1"),
	}

	val, _, err := Evaluate(logr.Discard(), ctx, condition)
	assert.Nil(t, err)
	assert.Equal(t, false, val, "expected to fail")
}

func Test_Condition_Messages(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"foo": "bar",
			"foo2": "bar2"
		}
	}
	`)

	ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
	err := context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}

	conditions := []kyverno.AnyAllConditions{
		{
			AnyConditions: []kyverno.Condition{
				{
					RawKey:   kyverno.ToJSON("{{request.object.metadata.name}}"),
					Operator: kyverno.ConditionOperators["Equal"],
					RawValue: kyverno.ToJSON("temp2"),
					Message:  "invalid name",
				},
				{
					RawKey:   kyverno.ToJSON("{{request.object.spec.foo}}"),
					Operator: kyverno.ConditionOperators["Equal"],
					RawValue: kyverno.ToJSON("bar2"),
					Message:  "invalid foo",
				},
			},
		},
	}

	val, msg, err := EvaluateAnyAllConditions(logr.Discard(), ctx, conditions)
	assert.Nil(t, err)
	assert.Equal(t, false, val)
	assert.Contains(t, msg, "invalid name; invalid foo")

	conditions[0].AnyConditions[0].RawValue = kyverno.ToJSON("temp")
	conditions[0].AnyConditions[1].RawValue = kyverno.ToJSON("bar")
	val, msg, err = EvaluateAnyAllConditions(logr.Discard(), ctx, conditions)
	assert.Nil(t, err)
	assert.Equal(t, true, val)
	assert.Equal(t, "invalid name", msg)

	conditions[0].AllConditions = append(conditions[0].AllConditions, conditions[0].AnyConditions[0])
	conditions[0].AllConditions = append(conditions[0].AllConditions, conditions[0].AnyConditions[1])
	conditions[0].AllConditions[1].RawValue = kyverno.ToJSON("bar2")

	val, msg, err = EvaluateAnyAllConditions(logr.Discard(), ctx, conditions)
	assert.Nil(t, err)
	assert.Equal(t, false, val)
	assert.Contains(t, msg, "invalid foo")

	conditions[0].AnyConditions[0].RawValue = kyverno.ToJSON("temp1")
	conditions[0].AnyConditions[1].RawValue = kyverno.ToJSON("bar2")
	conditions[0].AllConditions[1].Message = "invalid foo2"
	val, msg, err = EvaluateAnyAllConditions(logr.Discard(), ctx, conditions)
	assert.Nil(t, err)
	assert.Equal(t, false, val)
	assert.Contains(t, msg, "invalid name; invalid foo; invalid foo2")
}
