package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/json"
)

func TestCondition_Marshal(t *testing.T) {
	type fields struct {
		RawKey   *Any
		Operator ConditionOperator
		RawValue *Any
		Message  string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "empty",
			want: "{}",
		}, {
			name: "with key",
			fields: fields{
				RawKey: &Any{
					Value: "{{ request.object.name }}",
				},
				Operator: ConditionOperators["Equals"],
				RawValue: &Any{
					Value: "dummy",
				},
			},
			want: `{"key":"{{ request.object.name }}","operator":"Equals","value":"dummy"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Condition{
				RawKey:   tt.fields.RawKey,
				Operator: tt.fields.Operator,
				RawValue: tt.fields.RawValue,
				Message:  tt.fields.Message,
			}
			got, err := json.Marshal(c)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}
