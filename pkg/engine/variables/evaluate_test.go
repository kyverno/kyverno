package variables

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["Equals"], Value: "string"}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["Equals"], Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["Equals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["Equals"], Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["Equals"], Value: 1.0}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["Equals"], Value: true}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["Equals"], Value: false}, true},
		{kyverno.Condition{Key: "1024", Operator: kyverno.ConditionOperators["Equals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["Equals"], Value: "1024"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["Equals"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["Equals"], Value: "1024Mi"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["Equals"], Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["Equals"], Value: "60m"}, true},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.ConditionOperators["Equals"], Value: map[string]interface{}{"foo": "bar"}}, true},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.ConditionOperators["Equals"], Value: []interface{}{"foo", "bar"}}, true},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.ConditionOperators["Equals"], Value: []interface{}{map[string]string{"foo": "bar"}}}, true},
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["Equals"], Value: "not string"}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["Equals"], Value: 2}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["Equals"], Value: int64(2)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["Equals"], Value: 2}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["Equals"], Value: 2.0}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["Equals"], Value: false}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["Equals"], Value: true}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["Equals"], Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["Equals"], Value: "1024Mi"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["Equals"], Value: "5h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["Equals"], Value: "30m"}, false},
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["Equals"], Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["Equals"], Value: "2"}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["Equals"], Value: "2.0"}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["Equals"], Value: "false"}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["Equals"], Value: "true"}, false},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.ConditionOperators["Equals"], Value: map[string]interface{}{"bar": "foo"}}, false},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.ConditionOperators["Equals"], Value: []interface{}{"bar", "foo"}}, false},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.ConditionOperators["Equals"], Value: []interface{}{map[string]string{"bar": "foo"}}}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["Equals"], Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.ConditionOperators["Equals"], Value: 3600}, false},
		{kyverno.Condition{Key: "1.5.2", Operator: kyverno.ConditionOperators["Equals"], Value: "1.5.2"}, true},
		{kyverno.Condition{Key: "1.5.2", Operator: kyverno.ConditionOperators["Equals"], Value: "1.5.*"}, true},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["Equals"], Value: "1.5.5"}, false},

		// Not Equals
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["NotEquals"], Value: "string"}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["NotEquals"], Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["NotEquals"], Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["NotEquals"], Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["NotEquals"], Value: 1.0}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["NotEquals"], Value: false}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["NotEquals"], Value: false}, false},
		{kyverno.Condition{Key: "1024", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1Ki"}, false},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1024"}, false},
		{kyverno.Condition{Key: "1023", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1023"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1024Mi"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["NotEquals"], Value: "60m"}, false},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.ConditionOperators["NotEquals"], Value: map[string]interface{}{"foo": "bar"}}, false},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.ConditionOperators["NotEquals"], Value: []interface{}{"foo", "bar"}}, false},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.ConditionOperators["NotEquals"], Value: []interface{}{map[string]string{"foo": "bar"}}}, false},
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["NotEquals"], Value: "not string"}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["NotEquals"], Value: 2}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["NotEquals"], Value: int64(2)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["NotEquals"], Value: 2}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["NotEquals"], Value: 2.0}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["NotEquals"], Value: true}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["NotEquals"], Value: true}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["NotEquals"], Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1024Mi"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["NotEquals"], Value: "5h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["NotEquals"], Value: "30m"}, true},
		{kyverno.Condition{Key: "string", Operator: kyverno.ConditionOperators["NotEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["NotEquals"], Value: "2"}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["NotEquals"], Value: "2.0"}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.ConditionOperators["NotEquals"], Value: "false"}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.ConditionOperators["NotEquals"], Value: "true"}, true},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.ConditionOperators["NotEquals"], Value: map[string]interface{}{"bar": "foo"}}, true},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.ConditionOperators["NotEquals"], Value: []interface{}{"bar", "foo"}}, true},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.ConditionOperators["NotEquals"], Value: []interface{}{map[string]string{"bar": "foo"}}}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["NotEquals"], Value: 3600}, false},
		{kyverno.Condition{Key: "2h", Operator: kyverno.ConditionOperators["NotEquals"], Value: 3600}, true},
		{kyverno.Condition{Key: "1.5.2", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1.5.5"}, true},
		{kyverno.Condition{Key: "1.5.2", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1.5.*"}, false},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["NotEquals"], Value: "1.5.0"}, false},

		// Greater Than
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1.0}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 10}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1.0}, false},
		{kyverno.Condition{Key: "1025", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1023"}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1Mi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10Mi", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10Mi"}, false},
		{kyverno.Condition{Key: "10h", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "30m"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1h"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, true},
		{kyverno.Condition{Key: 100, Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10"}, true},
		{kyverno.Condition{Key: "100", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["GreaterThan"], Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThan"], Value: 3600}, false},
		{kyverno.Condition{Key: "2h", Operator: kyverno.ConditionOperators["GreaterThan"], Value: 3600}, true},
		{kyverno.Condition{Key: 3600, Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1h"}, false},
		{kyverno.Condition{Key: 3600, Operator: kyverno.ConditionOperators["GreaterThan"], Value: "30m"}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(10)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThan"], Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(1)}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: int64(10)}, false},
		{kyverno.Condition{Key: -5, Operator: kyverno.ConditionOperators["GreaterThan"], Value: 1}, false},
		{kyverno.Condition{Key: -5, Operator: kyverno.ConditionOperators["GreaterThan"], Value: -10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThan"], Value: -10}, true},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1.5.0"}, true},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["GreaterThan"], Value: "1.5.5"}, false},

		// Less Than
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["LessThan"], Value: 1.0}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: 10}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["LessThan"], Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["LessThan"], Value: 1.0}, false},
		{kyverno.Condition{Key: "1023", Operator: kyverno.ConditionOperators["LessThan"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["LessThan"], Value: "1025"}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["LessThan"], Value: "1Gi"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThan"], Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThan"], Value: "1Mi"}, false},
		{kyverno.Condition{Key: "1Mi", Operator: kyverno.ConditionOperators["LessThan"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10h", Operator: kyverno.ConditionOperators["LessThan"], Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThan"], Value: "30m"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThan"], Value: "1h"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThan"], Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: 100, Operator: kyverno.ConditionOperators["LessThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "100", Operator: kyverno.ConditionOperators["LessThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["LessThan"], Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["LessThan"], Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["LessThan"], Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: "10"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThan"], Value: 3600}, false},
		{kyverno.Condition{Key: "30m", Operator: kyverno.ConditionOperators["LessThan"], Value: 3600}, true},
		{kyverno.Condition{Key: 3600, Operator: kyverno.ConditionOperators["LessThan"], Value: "1h"}, false},
		{kyverno.Condition{Key: 3600, Operator: kyverno.ConditionOperators["LessThan"], Value: "30m"}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["LessThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThan"], Value: int64(10)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThan"], Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["LessThan"], Value: int64(1)}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: int64(10)}, true},
		{kyverno.Condition{Key: -5, Operator: kyverno.ConditionOperators["LessThan"], Value: 1}, true},
		{kyverno.Condition{Key: -5, Operator: kyverno.ConditionOperators["LessThan"], Value: -10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThan"], Value: -10}, false},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["LessThan"], Value: "1.5.0"}, false},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["LessThan"], Value: "1.5.5"}, true},

		// Greater Than or Equal
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1.0}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 10}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1.0}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: "1025", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1024", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1023"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1024"}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1Mi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10h", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "30m"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1h"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 100, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "100", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "10"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 3600}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(10)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: int64(10)}, false},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1.5.5"}, true},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1.5.0"}, true},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["GreaterThanOrEquals"], Value: "1.5.5"}, false},

		// Less Than or Equal
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1.0}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 10}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1.0}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1024"}, true},
		{kyverno.Condition{Key: "1024", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "1Ki", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1025"}, true},
		{kyverno.Condition{Key: "1023", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Ki"}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Gi"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Mi"}, false},
		{kyverno.Condition{Key: "1Mi", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10h", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "30m"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1h"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, false},
		{kyverno.Condition{Key: 100, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10"}, false},
		{kyverno.Condition{Key: "100", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "10"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 3600}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(10)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(1)}, true},
		{kyverno.Condition{Key: 10, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(1)}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: int64(10)}, true},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1.5.5"}, true},
		{kyverno.Condition{Key: "1.5.0", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1.5.5"}, true},
		{kyverno.Condition{Key: "1.5.5", Operator: kyverno.ConditionOperators["LessThanOrEquals"], Value: "1.5.0"}, false},

		// In
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["In"], Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["In"], Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["In"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},

		// Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["NotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},

		// Any In
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: "1.1.1.1"}, true},
		{kyverno.Condition{Key: []interface{}{"4.4.4.4", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"1.1.1.1"}}, true},
		{kyverno.Condition{Key: []interface{}{1, 2}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{5}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{1, 2, 3, 4}}, false},
		{kyverno.Condition{Key: []interface{}{"1*"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"5*"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"2*"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: []interface{}{"4*"}}, false},
		{kyverno.Condition{Key: []interface{}{"5"}, Operator: kyverno.ConditionOperators["AnyIn"], Value: "1-3"}, false},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["AnyIn"], Value: "1-10"}, true},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["AnyIn"], Value: "1-10"}, true},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.ConditionOperators["AnyIn"], Value: "7-10"}, false},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.ConditionOperators["AnyIn"], Value: "0-10"}, true},
		{kyverno.Condition{Key: []interface{}{1.002, 1.222}, Operator: kyverno.ConditionOperators["AnyIn"], Value: "1.001-10"}, true},

		// All In
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"4.4.4.4", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1"}}, false},
		{kyverno.Condition{Key: []interface{}{1, 2}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{1, 2, 3, 4}}, false},
		{kyverno.Condition{Key: []interface{}{5}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{1, 2, 3, 4}}, false},
		{kyverno.Condition{Key: []interface{}{"1*"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"5*"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"2.1.1.1", "2.2.2.2", "2.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"2*"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: []interface{}{"4*"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllIn"], Value: "5.5.5.5"}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["AllIn"], Value: "1-10"}, true},
		{kyverno.Condition{Key: []interface{}{3, 2}, Operator: kyverno.ConditionOperators["AllIn"], Value: "1-10"}, true},
		{kyverno.Condition{Key: []interface{}{3, 2}, Operator: kyverno.ConditionOperators["AllIn"], Value: "5-10"}, false},

		// All Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4", "1.1.1.1"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3", "1.1.1.1", "4.4.4.4"}}, false},
		{kyverno.Condition{Key: []interface{}{"5.5.5.5", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"7*", "6*", "5*"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1*", "2*"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "3.3.3.3", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"2*"}}, true},
		{kyverno.Condition{Key: []interface{}{"4.1.1.1", "4.2.2.2", "4.5.5.5"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: []interface{}{"4*"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: "2.2.2.2"}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.ConditionOperators["AllNotIn"], Value: "6-10"}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["AllNotIn"], Value: "1-6"}, false},
		{kyverno.Condition{Key: []interface{}{3, 2}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: "5-10"}, true},
		{kyverno.Condition{Key: []interface{}{2, 6}, Operator: kyverno.ConditionOperators["AllNotIn"], Value: "5-10"}, false},

		// Any Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"5.5.5.5", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: "4.4.4.4"}, true},
		{kyverno.Condition{Key: []interface{}{"5.5.5.5", "4.4.4.4"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1*", "3*", "5*"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2", "5.5.5.5"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"2*"}}, true},
		{kyverno.Condition{Key: []interface{}{"2.2*"}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: []interface{}{"2.2.2.2"}}, false},
		{kyverno.Condition{Key: "5", Operator: kyverno.ConditionOperators["AnyNotIn"], Value: "1-3"}, true},
		{kyverno.Condition{Key: []interface{}{1, 5, 11}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: "0-10"}, true},
		{kyverno.Condition{Key: []interface{}{1, 5, 7}, Operator: kyverno.ConditionOperators["AnyNotIn"], Value: "0-10"}, false},
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
		Operator: kyverno.ConditionOperators["Equal"],
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
		Operator: kyverno.ConditionOperators["Equal"],
		Value:    "temp1",
	}

	if Evaluate(log.Log, ctx, condition) {
		t.Error("expected to fail")
	}
}
