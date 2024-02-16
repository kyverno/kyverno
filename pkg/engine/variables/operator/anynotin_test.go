package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

func TestAnyNotInHandler_Evaluate(t *testing.T) {
	type fields struct {
		ctx context.EvalInterface
		log logr.Logger
	}
	type args struct {
		key   interface{}
		value interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "key is string and in value",
			args: args{
				key:   "kyverno",
				value: "kyverno",
			},
			want: false,
		},
		{
			name: "key is string and in value",
			args: args{
				key:   "kube-system",
				value: []interface{}{"default", "kube-*"},
			},
			want: false,
		},
		{
			name: "key is string and not in value",
			args: args{
				key:   "kyverno",
				value: "default",
			},
			want: true,
		},
		{
			name: "key is int and in value",
			args: args{
				key:   64,
				value: "64",
			},
			want: false,
		},
		{
			name: "key is int and in value",
			args: args{
				key:   1,
				value: []interface{}{1, 2, 3},
			},
			want: false,
		},
		{
			name: "key is int and not in value",
			args: args{
				key:   64,
				value: "default",
			},
			want: true,
		},
		{
			name: "key is float and in value",
			args: args{
				key:   3.14,
				value: "3.14",
			},
			want: false,
		},
		{
			name: "key is float and in value",
			args: args{
				key:   2.2,
				value: []interface{}{1.1, 2.2, 3.3},
			},
			want: false,
		},
		{
			name: "key is float and not in value",
			args: args{
				key:   3.14,
				value: "default",
			},
			want: true,
		},
		{
			name: "key is boolean and in value",
			args: args{
				key:   true,
				value: true,
			},
			want: false,
		},
		{
			name: "key is array and in value",
			args: args{
				key:   []interface{}{"kube-system", "kube-public"},
				value: "kube-*",
			},
			want: false,
		},
		{
			name: "key is array and partially in value",
			args: args{
				key:   []interface{}{"kube-system", "default"},
				value: "kube-system",
			},
			want: true,
		},
		{
			name: "key is array and not in value",
			args: args{
				key:   []interface{}{"default", "kyverno"},
				value: "kube-*",
			},
			want: true,
		},
		{
			name: "key is array of int and partially in value",
			args: args{
				key:   []interface{}{1, 2, 3},
				value: 2,
			},
			want: true,
		},
		{
			name: "key is array of int and not in value",
			args: args{
				key:   []interface{}{1, 2, 3},
				value: 4,
			},
			want: true,
		},
		{
			name: "key is array of float and partially in value",
			args: args{
				key:   []interface{}{1.1, 2.2, 3.3},
				value: 1.1,
			},
			want: true,
		},
		{
			name: "key is array of float and not in value",
			args: args{
				key:   []interface{}{1.1, 2.2, 3.3},
				value: 4.4,
			},
			want: true,
		},
		{
			name: "key is array of bool and partially in value",
			args: args{
				key:   []interface{}{true, false},
				value: true,
			},
			want: true,
		},
		{
			name: "key is an array of bool and not in value",
			args: args{
				key:   []interface{}{true},
				value: false,
			},
			want: true,
		},
		{
			name: "key and value are array and not in value",
			args: args{
				key:   []interface{}{"default", "kyverno"},
				value: []interface{}{"kube-*", "kube-system"},
			},
			want: true,
		},
		{
			name: "key and value are array and partially in value",
			args: args{
				key:   []interface{}{"default", "kyverno"},
				value: []interface{}{"kube-*", "ky*"},
			},
			want: true,
		},
		{
			name: "key and value are arrays, key is an empty array but is in value",
			args: args{
				key:   []interface{}{},
				value: []interface{}{"default", "kyverno"},
			},
			want: false,
		},
		{
			name: "key is an empty string and value is an array, key is not in value",
			args: args{
				key:   "",
				value: []interface{}{"default", "kyverno"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anynotin := AnyNotInHandler{
				ctx: tt.fields.ctx,
				log: tt.fields.log,
			}
			if got := anynotin.Evaluate(tt.args.key, tt.args.value); got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}
