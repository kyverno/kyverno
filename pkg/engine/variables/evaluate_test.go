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
		{kyverno.Condition{Key: "string", Operator: kyverno.Equals, Value: "string"}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.Equals, Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.Equals, Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.Equals, Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.Equals, Value: 1.0}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.Equals, Value: true}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.Equals, Value: false}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.Equals, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.Equals, Value: "1024Mi"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.Equals, Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.Equals, Value: "60m"}, true},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.Equals, Value: map[string]interface{}{"foo": "bar"}}, true},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.Equals, Value: []interface{}{"foo", "bar"}}, true},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.Equals, Value: []interface{}{map[string]string{"foo": "bar"}}}, true},
		{kyverno.Condition{Key: "string", Operator: kyverno.Equals, Value: "not string"}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.Equals, Value: 2}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.Equals, Value: int64(2)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.Equals, Value: 2}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.Equals, Value: 2.0}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.Equals, Value: false}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.Equals, Value: true}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.Equals, Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.Equals, Value: "1024Mi"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.Equals, Value: "5h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.Equals, Value: "30m"}, false},
		{kyverno.Condition{Key: "string", Operator: kyverno.Equals, Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.Equals, Value: "2"}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.Equals, Value: "2.0"}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.Equals, Value: "false"}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.Equals, Value: "true"}, false},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.Equals, Value: map[string]interface{}{"bar": "foo"}}, false},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.Equals, Value: []interface{}{"bar", "foo"}}, false},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.Equals, Value: []interface{}{map[string]string{"bar": "foo"}}}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.Equals, Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.Equals, Value: 3600}, false},

		// Not Equals
		{kyverno.Condition{Key: "string", Operator: kyverno.NotEquals, Value: "string"}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.NotEquals, Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.NotEquals, Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.NotEquals, Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.NotEquals, Value: 1.0}, false},
		{kyverno.Condition{Key: true, Operator: kyverno.NotEquals, Value: false}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.NotEquals, Value: false}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.NotEquals, Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.NotEquals, Value: "1024Mi"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.NotEquals, Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.NotEquals, Value: "60m"}, false},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.NotEquals, Value: map[string]interface{}{"foo": "bar"}}, false},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.NotEquals, Value: []interface{}{"foo", "bar"}}, false},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.NotEquals, Value: []interface{}{map[string]string{"foo": "bar"}}}, false},
		{kyverno.Condition{Key: "string", Operator: kyverno.NotEquals, Value: "not string"}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.NotEquals, Value: 2}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.NotEquals, Value: int64(2)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.NotEquals, Value: 2}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.NotEquals, Value: 2.0}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.NotEquals, Value: true}, false},
		{kyverno.Condition{Key: false, Operator: kyverno.NotEquals, Value: true}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.NotEquals, Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.NotEquals, Value: "1024Mi"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.NotEquals, Value: "5h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.NotEquals, Value: "30m"}, true},
		{kyverno.Condition{Key: "string", Operator: kyverno.NotEquals, Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.NotEquals, Value: "2"}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.NotEquals, Value: "2.0"}, true},
		{kyverno.Condition{Key: true, Operator: kyverno.NotEquals, Value: "false"}, true},
		{kyverno.Condition{Key: false, Operator: kyverno.NotEquals, Value: "true"}, true},
		{kyverno.Condition{Key: map[string]interface{}{"foo": "bar"}, Operator: kyverno.NotEquals, Value: map[string]interface{}{"bar": "foo"}}, true},
		{kyverno.Condition{Key: []interface{}{"foo", "bar"}, Operator: kyverno.NotEquals, Value: []interface{}{"bar", "foo"}}, true},
		{kyverno.Condition{Key: []interface{}{map[string]string{"foo": "bar"}}, Operator: kyverno.NotEquals, Value: []interface{}{map[string]string{"bar": "foo"}}}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.NotEquals, Value: 3600}, false},
		{kyverno.Condition{Key: "2h", Operator: kyverno.NotEquals, Value: 3600}, true},

		// Greater Than
		{kyverno.Condition{Key: 10, Operator: kyverno.GreaterThan, Value: 1}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.GreaterThan, Value: 1.0}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.GreaterThan, Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: 10}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.GreaterThan, Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.GreaterThan, Value: 1.0}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.GreaterThan, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThan, Value: "1Mi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThan, Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10Mi", Operator: kyverno.GreaterThan, Value: "10Mi"}, false},
		{kyverno.Condition{Key: "10h", Operator: kyverno.GreaterThan, Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThan, Value: "30m"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThan, Value: "1h"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThan, Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.GreaterThan, Value: 1}, true},
		{kyverno.Condition{Key: 100, Operator: kyverno.GreaterThan, Value: "10"}, true},
		{kyverno.Condition{Key: "100", Operator: kyverno.GreaterThan, Value: "10"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.GreaterThan, Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.GreaterThan, Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.GreaterThan, Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: "10"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThan, Value: 3600}, false},
		{kyverno.Condition{Key: "2h", Operator: kyverno.GreaterThan, Value: 3600}, true},
		{kyverno.Condition{Key: 3600, Operator: kyverno.GreaterThan, Value: "1h"}, false},
		{kyverno.Condition{Key: 3600, Operator: kyverno.GreaterThan, Value: "30m"}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.GreaterThan, Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThan, Value: int64(10)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThan, Value: 1}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.GreaterThan, Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThan, Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: 10, Operator: kyverno.GreaterThan, Value: int64(1)}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: int64(10)}, false},
		{kyverno.Condition{Key: -5, Operator: kyverno.GreaterThan, Value: 1}, false},
		{kyverno.Condition{Key: -5, Operator: kyverno.GreaterThan, Value: -10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThan, Value: -10}, true},

		// Less Than
		{kyverno.Condition{Key: 10, Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.LessThan, Value: 1.0}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: 10}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.LessThan, Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.LessThan, Value: 1.0}, false},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.LessThan, Value: "1Gi"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThan, Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThan, Value: "1Mi"}, false},
		{kyverno.Condition{Key: "1Mi", Operator: kyverno.LessThan, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10h", Operator: kyverno.LessThan, Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThan, Value: "30m"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThan, Value: "1h"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThan, Value: "1Gi"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: 100, Operator: kyverno.LessThan, Value: "10"}, false},
		{kyverno.Condition{Key: "100", Operator: kyverno.LessThan, Value: "10"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.LessThan, Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.LessThan, Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.LessThan, Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: "10"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThan, Value: 3600}, false},
		{kyverno.Condition{Key: "30m", Operator: kyverno.LessThan, Value: 3600}, true},
		{kyverno.Condition{Key: 3600, Operator: kyverno.LessThan, Value: "1h"}, false},
		{kyverno.Condition{Key: 3600, Operator: kyverno.LessThan, Value: "30m"}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.LessThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThan, Value: int64(10)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.LessThan, Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThan, Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: 10, Operator: kyverno.LessThan, Value: int64(1)}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: int64(10)}, true},
		{kyverno.Condition{Key: -5, Operator: kyverno.LessThan, Value: 1}, true},
		{kyverno.Condition{Key: -5, Operator: kyverno.LessThan, Value: -10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThan, Value: -10}, false},

		// Greater Than or Equal
		{kyverno.Condition{Key: 10, Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.GreaterThanOrEquals, Value: 1.0}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: 10}, false},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.GreaterThanOrEquals, Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: 1.5}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.GreaterThanOrEquals, Value: 1.0}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.GreaterThanOrEquals, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThanOrEquals, Value: "1Mi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThanOrEquals, Value: "10Gi"}, false},
		{kyverno.Condition{Key: "10h", Operator: kyverno.GreaterThanOrEquals, Value: "1h"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThanOrEquals, Value: "30m"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThanOrEquals, Value: "1h"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.GreaterThanOrEquals, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: 100, Operator: kyverno.GreaterThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "100", Operator: kyverno.GreaterThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.GreaterThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.GreaterThanOrEquals, Value: "10"}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.GreaterThanOrEquals, Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: "10"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.GreaterThanOrEquals, Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.GreaterThanOrEquals, Value: 3600}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.GreaterThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThanOrEquals, Value: int64(10)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.GreaterThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.GreaterThanOrEquals, Value: 10}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: 10, Operator: kyverno.GreaterThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.GreaterThanOrEquals, Value: int64(10)}, false},

		// Less Than or Equal
		{kyverno.Condition{Key: 10, Operator: kyverno.LessThanOrEquals, Value: 1}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.LessThanOrEquals, Value: 1.0}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.LessThanOrEquals, Value: 1}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: 10}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.LessThanOrEquals, Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: 1.5}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: 1.0, Operator: kyverno.LessThanOrEquals, Value: 1.0}, true},
		{kyverno.Condition{Key: "10Gi", Operator: kyverno.LessThanOrEquals, Value: "1Gi"}, false},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThanOrEquals, Value: "10Gi"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThanOrEquals, Value: "1Mi"}, false},
		{kyverno.Condition{Key: "1Mi", Operator: kyverno.LessThanOrEquals, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10h", Operator: kyverno.LessThanOrEquals, Value: "1h"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThanOrEquals, Value: "30m"}, false},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThanOrEquals, Value: "1h"}, true},
		{kyverno.Condition{Key: "1Gi", Operator: kyverno.LessThanOrEquals, Value: "1Gi"}, true},
		{kyverno.Condition{Key: "10", Operator: kyverno.LessThanOrEquals, Value: 1}, false},
		{kyverno.Condition{Key: 100, Operator: kyverno.LessThanOrEquals, Value: "10"}, false},
		{kyverno.Condition{Key: "100", Operator: kyverno.LessThanOrEquals, Value: "10"}, false},
		{kyverno.Condition{Key: "10", Operator: kyverno.LessThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.LessThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.LessThanOrEquals, Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: "10"}, true},
		{kyverno.Condition{Key: "1h", Operator: kyverno.LessThanOrEquals, Value: 3600}, true},
		{kyverno.Condition{Key: "2h", Operator: kyverno.LessThanOrEquals, Value: 3600}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.LessThanOrEquals, Value: int64(1)}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThanOrEquals, Value: int64(10)}, true},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThanOrEquals, Value: 1}, true},
		{kyverno.Condition{Key: int64(10), Operator: kyverno.LessThanOrEquals, Value: 1}, false},
		{kyverno.Condition{Key: int64(1), Operator: kyverno.LessThanOrEquals, Value: 10}, true},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: int64(1)}, true},
		{kyverno.Condition{Key: 10, Operator: kyverno.LessThanOrEquals, Value: int64(1)}, false},
		{kyverno.Condition{Key: 1, Operator: kyverno.LessThanOrEquals, Value: int64(10)}, true},

		// In
		{kyverno.Condition{Key: 1, Operator: kyverno.In, Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.In, Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "1", Operator: kyverno.In, Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.In, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: 5, Operator: kyverno.In, Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.In, Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "5", Operator: kyverno.In, Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.In, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},

		// Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.NotIn, Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.NotIn, Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.NotIn, Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.NotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.NotIn, Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.NotIn, Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.NotIn, Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.NotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},

		// Any In
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.AnyIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"4.4.4.4", "5.5.5.5"}, Operator: kyverno.AnyIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.AnyIn, Value: []interface{}{"1.1.1.1"}}, true},
		{kyverno.Condition{Key: []interface{}{1, 2}, Operator: kyverno.AnyIn, Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.AnyIn, Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{5}, Operator: kyverno.AnyIn, Value: []interface{}{1, 2, 3, 4}}, false},

		// All In
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.AllIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.AllIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"4.4.4.4", "5.5.5.5"}, Operator: kyverno.AllIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "5.5.5.5"}, Operator: kyverno.AllIn, Value: []interface{}{"1.1.1.1"}}, false},
		{kyverno.Condition{Key: []interface{}{1, 2}, Operator: kyverno.AllIn, Value: []interface{}{1, 2, 3, 4}}, true},
		{kyverno.Condition{Key: []interface{}{1, 5}, Operator: kyverno.AllIn, Value: []interface{}{1, 2, 3, 4}}, false},
		{kyverno.Condition{Key: []interface{}{5}, Operator: kyverno.AllIn, Value: []interface{}{1, 2, 3, 4}}, false},

		// All Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.AllNotIn, Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.AllNotIn, Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.AllNotIn, Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.AllNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.AllNotIn, Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.AllNotIn, Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.AllNotIn, Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.AllNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: []interface{}{"5.5.5.5", "4.4.4.4"}, Operator: kyverno.AllNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},

		// Any Not In
		{kyverno.Condition{Key: 1, Operator: kyverno.AnyNotIn, Value: []interface{}{1, 2, 3}}, false},
		{kyverno.Condition{Key: 1.5, Operator: kyverno.AnyNotIn, Value: []interface{}{1, 1.5, 2, 3}}, false},
		{kyverno.Condition{Key: "1", Operator: kyverno.AnyNotIn, Value: []interface{}{"1", "2", "3"}}, false},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "2.2.2.2"}, Operator: kyverno.AnyNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, false},
		{kyverno.Condition{Key: 5, Operator: kyverno.AnyNotIn, Value: []interface{}{1, 2, 3}}, true},
		{kyverno.Condition{Key: 5.5, Operator: kyverno.AnyNotIn, Value: []interface{}{1, 1.5, 2, 3}}, true},
		{kyverno.Condition{Key: "5", Operator: kyverno.AnyNotIn, Value: []interface{}{"1", "2", "3"}}, true},
		{kyverno.Condition{Key: []interface{}{"1.1.1.1", "4.4.4.4"}, Operator: kyverno.AnyNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
		{kyverno.Condition{Key: []interface{}{"5.5.5.5", "4.4.4.4"}, Operator: kyverno.AnyNotIn, Value: []interface{}{"1.1.1.1", "2.2.2.2", "3.3.3.3"}}, true},
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
