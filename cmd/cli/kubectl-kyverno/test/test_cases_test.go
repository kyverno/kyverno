package test

import (
	"errors"
	"reflect"
	"testing"
)

func TestTestCases_Errors(t *testing.T) {
	tests := []struct {
		name string
		tc   TestCases
		want []TestCase
	}{{
		name: "nil",
		tc:   nil,
		want: nil,
	}, {
		name: "empty",
		tc:   []TestCase{},
		want: nil,
	}, {
		name: "no error",
		tc:   TestCases([]TestCase{{}}),
		want: nil,
	}, {
		name: "one error",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}},
		want: []TestCase{{
			Err: errors.New("error 1"),
		}},
	}, {
		name: "two errors",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}, {
			Err: errors.New("error 2"),
		}},
		want: []TestCase{{
			Err: errors.New("error 1"),
		}, {
			Err: errors.New("error 2"),
		}},
	}, {
		name: "mixed",
		tc: []TestCase{{
			Err: errors.New("error 1"),
		}, {}, {
			Err: errors.New("error 2"),
		}, {}},
		want: []TestCase{{
			Err: errors.New("error 1"),
		}, {
			Err: errors.New("error 2"),
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tc.Errors(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestCases.Errors() = %v, want %v", got, tt.want)
			}
		})
	}
}
