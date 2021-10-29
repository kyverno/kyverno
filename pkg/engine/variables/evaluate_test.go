package variables

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestEvaluate(t *testing.T) {
	testCases := []struct {
		Condition kyverno.Condition
		Result    bool
	}{
		// Equals
		{kyverno.Condition{"string", kyverno.Equals, "string"}, true},
		{kyverno.Condition{1, kyverno.Equals, 1}, true},
		{kyverno.Condition{int64(1), kyverno.Equals, int64(1)}, true},
		{kyverno.Condition{int64(1), kyverno.Equals, 1}, true},
		{kyverno.Condition{1.0, kyverno.Equals, 1.0}, true},
		{kyverno.Condition{true, kyverno.Equals, true}, true},
		{kyverno.Condition{false, kyverno.Equals, false}, true},
		{kyverno.Condition{"1Gi", kyverno.Equals, "1Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.Equals, "1024Mi"}, true},
		{kyverno.Condition{"1h", kyverno.Equals, "1h"}, true},
		{kyverno.Condition{"1h", kyverno.Equals, "60m"}, true},
		{kyverno.Condition{map[string]interface{}{"foo": "bar"}, kyverno.Equals, map[string]interface{}{"foo": "bar"}}, true},
		{kyverno.Condition{[]interface{}{"foo", "bar"}, kyverno.Equals, []interface{}{"foo", "bar"}}, true},
		{kyverno.Condition{[]interface{}{map[string]string{"foo": "bar"}}, kyverno.Equals, []interface{}{map[string]string{"foo": "bar"}}}, true},
		{kyverno.Condition{"string", kyverno.Equals, "not string"}, false},
		{kyverno.Condition{1, kyverno.Equals, 2}, false},
		{kyverno.Condition{int64(1), kyverno.Equals, int64(2)}, false},
		{kyverno.Condition{int64(1), kyverno.Equals, 2}, false},
		{kyverno.Condition{1.0, kyverno.Equals, 2.0}, false},
		{kyverno.Condition{true, kyverno.Equals, false}, false},
		{kyverno.Condition{false, kyverno.Equals, true}, false},
		{kyverno.Condition{"1Gi", kyverno.Equals, "10Gi"}, false},
		{kyverno.Condition{"10Gi", kyverno.Equals, "1024Mi"}, false},
		{kyverno.Condition{"1h", kyverno.Equals, "5h"}, false},
		{kyverno.Condition{"1h", kyverno.Equals, "30m"}, false},
		{kyverno.Condition{"string", kyverno.Equals, 1}, false},
		{kyverno.Condition{1, kyverno.Equals, "2"}, false},
		{kyverno.Condition{1.0, kyverno.Equals, "2.0"}, false},
		{kyverno.Condition{true, kyverno.Equals, "false"}, false},
		{kyverno.Condition{false, kyverno.Equals, "true"}, false},
		{kyverno.Condition{map[string]interface{}{"foo": "bar"}, kyverno.Equals, map[string]interface{}{"bar": "foo"}}, false},
		{kyverno.Condition{[]interface{}{"foo", "bar"}, kyverno.Equals, []interface{}{"bar", "foo"}}, false},
		{kyverno.Condition{[]interface{}{map[string]string{"foo": "bar"}}, kyverno.Equals, []interface{}{map[string]string{"bar": "foo"}}}, false},
		{kyverno.Condition{"1h", kyverno.Equals, 3600}, true},
		{kyverno.Condition{"2h", kyverno.Equals, 3600}, false},

		// Not Equals
		{kyverno.Condition{"string", kyverno.NotEquals, "string"}, false},
		{kyverno.Condition{1, kyverno.NotEquals, 1}, false},
		{kyverno.Condition{int64(1), kyverno.NotEquals, int64(1)}, false},
		{kyverno.Condition{int64(1), kyverno.NotEquals, 1}, false},
		{kyverno.Condition{1.0, kyverno.NotEquals, 1.0}, false},
		{kyverno.Condition{true, kyverno.NotEquals, false}, true},
		{kyverno.Condition{false, kyverno.NotEquals, false}, false},
		{kyverno.Condition{"1Gi", kyverno.NotEquals, "1Gi"}, false},
		{kyverno.Condition{"10Gi", kyverno.NotEquals, "1024Mi"}, true},
		{kyverno.Condition{"1h", kyverno.NotEquals, "1h"}, false},
		{kyverno.Condition{"1h", kyverno.NotEquals, "60m"}, false},
		{kyverno.Condition{map[string]interface{}{"foo": "bar"}, kyverno.NotEquals, map[string]interface{}{"foo": "bar"}}, false},
		{kyverno.Condition{[]interface{}{"foo", "bar"}, kyverno.NotEquals, []interface{}{"foo", "bar"}}, false},
		{kyverno.Condition{[]interface{}{map[string]string{"foo": "bar"}}, kyverno.NotEquals, []interface{}{map[string]string{"foo": "bar"}}}, false},
		{kyverno.Condition{"string", kyverno.NotEquals, "not string"}, true},
		{kyverno.Condition{1, kyverno.NotEquals, 2}, true},
		{kyverno.Condition{int64(1), kyverno.NotEquals, int64(2)}, true},
		{kyverno.Condition{int64(1), kyverno.NotEquals, 2}, true},
		{kyverno.Condition{1.0, kyverno.NotEquals, 2.0}, true},
		{kyverno.Condition{true, kyverno.NotEquals, true}, false},
		{kyverno.Condition{false, kyverno.NotEquals, true}, true},
		{kyverno.Condition{"1Gi", kyverno.NotEquals, "10Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.NotEquals, "1024Mi"}, false},
		{kyverno.Condition{"1h", kyverno.NotEquals, "5h"}, true},
		{kyverno.Condition{"1h", kyverno.NotEquals, "30m"}, true},
		{kyverno.Condition{"string", kyverno.NotEquals, 1}, true},
		{kyverno.Condition{1, kyverno.NotEquals, "2"}, true},
		{kyverno.Condition{1.0, kyverno.NotEquals, "2.0"}, true},
		{kyverno.Condition{true, kyverno.NotEquals, "false"}, true},
		{kyverno.Condition{false, kyverno.NotEquals, "true"}, true},
		{kyverno.Condition{map[string]interface{}{"foo": "bar"}, kyverno.NotEquals, map[string]interface{}{"bar": "foo"}}, true},
		{kyverno.Condition{[]interface{}{"foo", "bar"}, kyverno.NotEquals, []interface{}{"bar", "foo"}}, true},
		{kyverno.Condition{[]interface{}{map[string]string{"foo": "bar"}}, kyverno.NotEquals, []interface{}{map[string]string{"bar": "foo"}}}, true},
		{kyverno.Condition{"1h", kyverno.NotEquals, 3600}, false},
		{kyverno.Condition{"2h", kyverno.NotEquals, 3600}, true},

		// Greater Than
		{kyverno.Condition{10, kyverno.GreaterThan, 1}, true},
		{kyverno.Condition{1.5, kyverno.GreaterThan, 1.0}, true},
		{kyverno.Condition{1.5, kyverno.GreaterThan, 1}, true},
		{kyverno.Condition{1, kyverno.GreaterThan, 10}, false},
		{kyverno.Condition{1.0, kyverno.GreaterThan, 1.5}, false},
		{kyverno.Condition{1, kyverno.GreaterThan, 1.5}, false},
		{kyverno.Condition{1, kyverno.GreaterThan, 1}, false},
		{kyverno.Condition{1.0, kyverno.GreaterThan, 1.0}, false},
		{kyverno.Condition{"10Gi", kyverno.GreaterThan, "1Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.GreaterThan, "1Mi"}, true},
		{kyverno.Condition{"1Gi", kyverno.GreaterThan, "10Gi"}, false},
		{kyverno.Condition{"10Mi", kyverno.GreaterThan, "10Mi"}, false},
		{kyverno.Condition{"10h", kyverno.GreaterThan, "1h"}, true},
		{kyverno.Condition{"1h", kyverno.GreaterThan, "30m"}, true},
		{kyverno.Condition{"1h", kyverno.GreaterThan, "1h"}, false},
		{kyverno.Condition{"1Gi", kyverno.GreaterThan, "1Gi"}, false},
		{kyverno.Condition{"10", kyverno.GreaterThan, 1}, true},
		{kyverno.Condition{100, kyverno.GreaterThan, "10"}, true},
		{kyverno.Condition{"100", kyverno.GreaterThan, "10"}, true},
		{kyverno.Condition{"10", kyverno.GreaterThan, "10"}, false},
		{kyverno.Condition{"1", kyverno.GreaterThan, "10"}, false},
		{kyverno.Condition{"1", kyverno.GreaterThan, 10}, false},
		{kyverno.Condition{1, kyverno.GreaterThan, "10"}, false},
		{kyverno.Condition{"1h", kyverno.GreaterThan, 3600}, false},
		{kyverno.Condition{"2h", kyverno.GreaterThan, 3600}, true},
		{kyverno.Condition{3600, kyverno.GreaterThan, "1h"}, false},
		{kyverno.Condition{3600, kyverno.GreaterThan, "30m"}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThan, int64(1)}, false},
		{kyverno.Condition{int64(10), kyverno.GreaterThan, int64(1)}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThan, int64(10)}, false},
		{kyverno.Condition{int64(1), kyverno.GreaterThan, 1}, false},
		{kyverno.Condition{int64(10), kyverno.GreaterThan, 1}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThan, 10}, false},
		{kyverno.Condition{1, kyverno.GreaterThan, int64(1)}, false},
		{kyverno.Condition{10, kyverno.GreaterThan, int64(1)}, true},
		{kyverno.Condition{1, kyverno.GreaterThan, int64(10)}, false},
		{kyverno.Condition{-5, kyverno.GreaterThan, 1}, false},
		{kyverno.Condition{-5, kyverno.GreaterThan, -10}, true},
		{kyverno.Condition{1, kyverno.GreaterThan, -10}, true},

		// Less Than
		{kyverno.Condition{10, kyverno.LessThan, 1}, false},
		{kyverno.Condition{1.5, kyverno.LessThan, 1.0}, false},
		{kyverno.Condition{1.5, kyverno.LessThan, 1}, false},
		{kyverno.Condition{1, kyverno.LessThan, 10}, true},
		{kyverno.Condition{1.0, kyverno.LessThan, 1.5}, true},
		{kyverno.Condition{1, kyverno.LessThan, 1.5}, true},
		{kyverno.Condition{1, kyverno.LessThan, 1}, false},
		{kyverno.Condition{1.0, kyverno.LessThan, 1.0}, false},
		{kyverno.Condition{"10Gi", kyverno.LessThan, "1Gi"}, false},
		{kyverno.Condition{"1Gi", kyverno.LessThan, "10Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.LessThan, "1Mi"}, false},
		{kyverno.Condition{"1Mi", kyverno.LessThan, "1Gi"}, true},
		{kyverno.Condition{"10h", kyverno.LessThan, "1h"}, false},
		{kyverno.Condition{"1h", kyverno.LessThan, "30m"}, false},
		{kyverno.Condition{"1h", kyverno.LessThan, "1h"}, false},
		{kyverno.Condition{"1Gi", kyverno.LessThan, "1Gi"}, false},
		{kyverno.Condition{"10", kyverno.LessThan, 1}, false},
		{kyverno.Condition{100, kyverno.LessThan, "10"}, false},
		{kyverno.Condition{"100", kyverno.LessThan, "10"}, false},
		{kyverno.Condition{"10", kyverno.LessThan, "10"}, false},
		{kyverno.Condition{"1", kyverno.LessThan, "10"}, true},
		{kyverno.Condition{"1", kyverno.LessThan, 10}, true},
		{kyverno.Condition{1, kyverno.LessThan, "10"}, true},
		{kyverno.Condition{"1h", kyverno.LessThan, 3600}, false},
		{kyverno.Condition{"30m", kyverno.LessThan, 3600}, true},
		{kyverno.Condition{3600, kyverno.LessThan, "1h"}, false},
		{kyverno.Condition{3600, kyverno.LessThan, "30m"}, false},
		{kyverno.Condition{int64(1), kyverno.LessThan, int64(1)}, false},
		{kyverno.Condition{int64(10), kyverno.LessThan, int64(1)}, false},
		{kyverno.Condition{int64(1), kyverno.LessThan, int64(10)}, true},
		{kyverno.Condition{int64(1), kyverno.LessThan, 1}, false},
		{kyverno.Condition{int64(10), kyverno.LessThan, 1}, false},
		{kyverno.Condition{int64(1), kyverno.LessThan, 10}, true},
		{kyverno.Condition{1, kyverno.LessThan, int64(1)}, false},
		{kyverno.Condition{10, kyverno.LessThan, int64(1)}, false},
		{kyverno.Condition{1, kyverno.LessThan, int64(10)}, true},
		{kyverno.Condition{-5, kyverno.LessThan, 1}, true},
		{kyverno.Condition{-5, kyverno.LessThan, -10}, false},
		{kyverno.Condition{1, kyverno.LessThan, -10}, false},

		// Greater Than or Equal
		{kyverno.Condition{10, kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{1.5, kyverno.GreaterThanOrEquals, 1.0}, true},
		{kyverno.Condition{1.5, kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, 10}, false},
		{kyverno.Condition{1.0, kyverno.GreaterThanOrEquals, 1.5}, false},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, 1.5}, false},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{1.0, kyverno.GreaterThanOrEquals, 1.0}, true},
		{kyverno.Condition{1.0, kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{"10Gi", kyverno.GreaterThanOrEquals, "1Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.GreaterThanOrEquals, "1Mi"}, true},
		{kyverno.Condition{"1Gi", kyverno.GreaterThanOrEquals, "10Gi"}, false},
		{kyverno.Condition{"10h", kyverno.GreaterThanOrEquals, "1h"}, true},
		{kyverno.Condition{"1h", kyverno.GreaterThanOrEquals, "30m"}, true},
		{kyverno.Condition{"1h", kyverno.GreaterThanOrEquals, "1h"}, true},
		{kyverno.Condition{"1Gi", kyverno.GreaterThanOrEquals, "1Gi"}, true},
		{kyverno.Condition{"10", kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{100, kyverno.GreaterThanOrEquals, "10"}, true},
		{kyverno.Condition{"100", kyverno.GreaterThanOrEquals, "10"}, true},
		{kyverno.Condition{"10", kyverno.GreaterThanOrEquals, "10"}, true},
		{kyverno.Condition{"1", kyverno.GreaterThanOrEquals, "10"}, false},
		{kyverno.Condition{"1", kyverno.GreaterThanOrEquals, 10}, false},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, "10"}, false},
		{kyverno.Condition{"1h", kyverno.GreaterThanOrEquals, 3600}, true},
		{kyverno.Condition{"2h", kyverno.GreaterThanOrEquals, 3600}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThanOrEquals, int64(1)}, true},
		{kyverno.Condition{int64(10), kyverno.GreaterThanOrEquals, int64(1)}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThanOrEquals, int64(10)}, false},
		{kyverno.Condition{int64(1), kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{int64(10), kyverno.GreaterThanOrEquals, 1}, true},
		{kyverno.Condition{int64(1), kyverno.GreaterThanOrEquals, 10}, false},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, int64(1)}, true},
		{kyverno.Condition{10, kyverno.GreaterThanOrEquals, int64(1)}, true},
		{kyverno.Condition{1, kyverno.GreaterThanOrEquals, int64(10)}, false},

		// Less Than or Equal
		{kyverno.Condition{10, kyverno.LessThanOrEquals, 1}, false},
		{kyverno.Condition{1.5, kyverno.LessThanOrEquals, 1.0}, false},
		{kyverno.Condition{1.5, kyverno.LessThanOrEquals, 1}, false},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, 10}, true},
		{kyverno.Condition{1.0, kyverno.LessThanOrEquals, 1.5}, true},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, 1.5}, true},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, 1}, true},
		{kyverno.Condition{1.0, kyverno.LessThanOrEquals, 1.0}, true},
		{kyverno.Condition{"10Gi", kyverno.LessThanOrEquals, "1Gi"}, false},
		{kyverno.Condition{"1Gi", kyverno.LessThanOrEquals, "10Gi"}, true},
		{kyverno.Condition{"1Gi", kyverno.LessThanOrEquals, "1Mi"}, false},
		{kyverno.Condition{"1Mi", kyverno.LessThanOrEquals, "1Gi"}, true},
		{kyverno.Condition{"10h", kyverno.LessThanOrEquals, "1h"}, false},
		{kyverno.Condition{"1h", kyverno.LessThanOrEquals, "30m"}, false},
		{kyverno.Condition{"1h", kyverno.LessThanOrEquals, "1h"}, true},
		{kyverno.Condition{"1Gi", kyverno.LessThanOrEquals, "1Gi"}, true},
		{kyverno.Condition{"10", kyverno.LessThanOrEquals, 1}, false},
		{kyverno.Condition{100, kyverno.LessThanOrEquals, "10"}, false},
		{kyverno.Condition{"100", kyverno.LessThanOrEquals, "10"}, false},
		{kyverno.Condition{"10", kyverno.LessThanOrEquals, "10"}, true},
		{kyverno.Condition{"1", kyverno.LessThanOrEquals, "10"}, true},
		{kyverno.Condition{"1", kyverno.LessThanOrEquals, 10}, true},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, "10"}, true},
		{kyverno.Condition{"1h", kyverno.LessThanOrEquals, 3600}, true},
		{kyverno.Condition{"2h", kyverno.LessThanOrEquals, 3600}, false},
		{kyverno.Condition{int64(1), kyverno.LessThanOrEquals, int64(1)}, true},
		{kyverno.Condition{int64(10), kyverno.LessThanOrEquals, int64(1)}, false},
		{kyverno.Condition{int64(1), kyverno.LessThanOrEquals, int64(10)}, true},
		{kyverno.Condition{int64(1), kyverno.LessThanOrEquals, 1}, true},
		{kyverno.Condition{int64(10), kyverno.LessThanOrEquals, 1}, false},
		{kyverno.Condition{int64(1), kyverno.LessThanOrEquals, 10}, true},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, int64(1)}, true},
		{kyverno.Condition{10, kyverno.LessThanOrEquals, int64(1)}, false},
		{kyverno.Condition{1, kyverno.LessThanOrEquals, int64(10)}, true},

		// In
		{kyverno.Condition{1, kyverno.In, []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{1.5, kyverno.In, []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{"1", kyverno.In, []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "2.2.2.2"}, kyverno.In, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{5, kyverno.In, []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{5.5, kyverno.In, []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{"5", kyverno.In, []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "4.4.4.4"}, kyverno.In, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},

		// Not In
		{kyverno.Condition{1, kyverno.NotIn, []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{1.5, kyverno.NotIn, []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{"1", kyverno.NotIn, []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "2.2.2.2"}, kyverno.NotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{5, kyverno.NotIn, []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{5.5, kyverno.NotIn, []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{"5", kyverno.NotIn, []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "4.4.4.4"}, kyverno.NotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},

		// Any In
		{kyverno.Condition{[]interface{}{"1.1.1.1", "5.5.5.5"}, kyverno.AnyIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{[]interface{}{"4.4.4.4", "5.5.5.5"}, kyverno.AnyIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "5.5.5.5"}, kyverno.AnyIn, []interface{}{"1.1.1.1"}}, true},
		{kyverno.Condition{[]interface{}{1, 2}, kyverno.AnyIn, []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{[]interface{}{1, 5}, kyverno.AnyIn, []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{[]interface{}{5}, kyverno.AnyIn, []interface{}{1, 2, 3, 4}}, false},

		// All In
		{kyverno.Condition{[]interface{}{"1.1.1.1", "2.2.2.2"}, kyverno.AllIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "5.5.5.5"}, kyverno.AllIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{[]interface{}{"4.4.4.4", "5.5.5.5"}, kyverno.AllIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "5.5.5.5"}, kyverno.AllIn, []interface{}{"1.1.1.1"}}, false},
		{kyverno.Condition{[]interface{}{1, 2}, kyverno.AllIn, []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{[]interface{}{1, 5}, kyverno.AllIn, []interface{}{1, 2, 3, 4}}, false},
		{kyverno.Condition{[]interface{}{5}, kyverno.AllIn, []interface{}{1, 2, 3, 4}}, false},

		// All Not In
		{kyverno.Condition{1, kyverno.AllNotIn, []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{1.5, kyverno.AllNotIn, []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{"1", kyverno.AllNotIn, []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "2.2.2.2"}, kyverno.AllNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{5, kyverno.AllNotIn, []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{5.5, kyverno.AllNotIn, []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{"5", kyverno.AllNotIn, []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "4.4.4.4"}, kyverno.AllNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{[]interface{}{"5.5.5.5", "4.4.4.4"}, kyverno.AllNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},

		// Any Not In
		{kyverno.Condition{1, kyverno.AnyNotIn, []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{1.5, kyverno.AnyNotIn, []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{"1", kyverno.AnyNotIn, []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "2.2.2.2"}, kyverno.AnyNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{5, kyverno.AnyNotIn, []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{5.5, kyverno.AnyNotIn, []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{"5", kyverno.AnyNotIn, []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{[]interface{}{"1.1.1.1", "4.4.4.4"}, kyverno.AnyNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{[]interface{}{"5.5.5.5", "4.4.4.4"}, kyverno.AnyNotIn, []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
	}

	ctx := context.NewContext()
	for _, tc := range testCases {
		if Evaluate(log.Log, ctx, tc.Condition) != tc.Result {
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

	conditionJSON, err := json.Marshal(condition)
	assert.Nil(t, err)

	var conditionMap interface{}
	err = json.Unmarshal(conditionJSON, &conditionMap)
	assert.Nil(t, err)

	conditionWithResolvedVars, _ := SubstituteAllInPreconditions(log.Log, ctx, conditionMap)
	conditionJSON, err = json.Marshal(conditionWithResolvedVars)
	assert.Nil(t, err)

	err = json.Unmarshal(conditionJSON, &condition)
	assert.Nil(t, err)
	assert.True(t, Evaluate(log.Log, ctx, condition))
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
