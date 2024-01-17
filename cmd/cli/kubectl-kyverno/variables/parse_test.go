package variables

import (
	"reflect"
	"testing"
)

func Test_parse(t *testing.T) {
	tests := []struct {
		name string
		vars []string
		want map[string]string
	}{{
		name: "nil",
		vars: nil,
		want: nil,
	}, {
		name: "empty",
		vars: []string{},
		want: nil,
	}, {
		name: "request.object",
		vars: []string{
			"request.object.spec=something",
		},
		want: nil,
	}, {
		name: "duplicate",
		vars: []string{
			"foo=something",
			"foo=something-else",
		},
		want: map[string]string{
			"foo": "something",
		},
	}, {
		name: "invalid",
		vars: []string{
			"foo",
		},
		want: nil,
	}, {
		name: "valid",
		vars: []string{
			"object.data=123",
		},
		want: map[string]string{
			"object.data": "123",
		},
	}, {
		name: "valid",
		vars: []string{
			"object.data=123",
			"object.spec=abc",
		},
		want: map[string]string{
			"object.data": "123",
			"object.spec": "abc",
		},
	}, {
		name: "mixed",
		vars: []string{
			"object.data=123",
			"bar",
			"foo=",
			"object.spec=abc",
			"=baz",
		},
		want: map[string]string{
			"object.data": "123",
			"object.spec": "abc",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parse(tt.vars...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
