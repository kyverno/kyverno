package operator

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/context"
)

func TestAnyInHandler_Evaluate(t *testing.T) {
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
			want: true,
		},
		{
			name: "key is string and in value with wildcard",
			args: args{
				key:   "kube-system",
				value: []interface{}{"default", "kube-*"},
			},
			want: true,
		},
		{
			name: "key is string and not in value",
			args: args{
				key:   "kyverno",
				value: "default",
			},
			want: false,
		},
		{
			name: "key is int and in value",
			args: args{
				key:   64,
				value: "64",
			},
			want: true,
		},
		{
			name: "key is int and in value slice",
			args: args{
				key:   1,
				value: []interface{}{1, 2, 3},
			},
			want: true,
		},
		{
			name: "key is int and not in value",
			args: args{
				key:   64,
				value: "default",
			},
			want: false,
		},
		{
			name: "key is float and in value",
			args: args{
				key:   3.14,
				value: "3.14",
			},
			want: true,
		},
		{
			name: "key is float and in value slice",
			args: args{
				key:   2.2,
				value: []interface{}{1.1, 2.2, 3.3},
			},
			want: true,
		},
		{
			name: "key is float and not in value",
			args: args{
				key:   3.14,
				value: "default",
			},
			want: false,
		},
		{
			name: "key is boolean and in value",
			args: args{
				key:   true,
				value: "true",
			},
			want: true,
		},
		{
			name: "key is array and all in value with wildcard",
			args: args{
				key:   []interface{}{"kube-system", "kube-public"},
				value: "kube-*",
			},
			want: true,
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
			want: false,
		},
		{
			name: "key and value are array and any in value",
			args: args{
				key:   []interface{}{"default", "kyverno"},
				value: []interface{}{"kube-*", "ky*"},
			},
			want: true,
		},
		{
			name: "key and value are array and none in value",
			args: args{
				key:   []interface{}{"default", "kyverno"},
				value: []interface{}{"kube-*", "kube-system"},
			},
			want: false,
		},
		{
			name: "key is an empty array",
			args: args{
				key:   []interface{}{},
				value: []interface{}{"default", "kyverno"},
			},
			want: false,
		},
		{
			name: "key is an empty string and value is an array",
			args: args{
				key:   "",
				value: []interface{}{"default", "kyverno"},
			},
			want: false,
		},
		{
			name: "unsupported key type",
			args: args{
				key:   map[string]interface{}{"foo": "bar"},
				value: "test",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anyin := AnyInHandler{
				ctx: tt.fields.ctx,
				log: tt.fields.log,
			}
			if got := anyin.Evaluate(tt.args.key, tt.args.value); got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}
