package wildcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsWildcard(t *testing.T) {
	type args struct {
		v string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "no wildcard",
		args: args{
			v: "name",
		},
		want: false,
	}, {
		name: "empty string",
		args: args{
			v: "",
		},
		want: false,
	}, {
		name: "contains * at the end",
		args: args{
			v: "name*",
		},
		want: true,
	}, {
		name: "contains * at the beginning",
		args: args{
			v: "*name",
		},
		want: true,
	}, {
		name: "contains * in the middle",
		args: args{
			v: "start*end",
		},
		want: true,
	}, {
		name: "only *",
		args: args{
			v: "*",
		},
		want: true,
	}, {
		name: "contains ? at the end",
		args: args{
			v: "name?",
		},
		want: true,
	}, {
		name: "contains ? at the beginning",
		args: args{
			v: "?name",
		},
		want: true,
	}, {
		name: "contains ? in the middle",
		args: args{
			v: "start?end",
		},
		want: true,
	}, {
		name: "only ?",
		args: args{
			v: "?",
		},
		want: true,
	}, {
		name: "both * and ?",
		args: args{
			v: "*name?",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ContainsWildcard(tt.args.v))
		})
	}
}
