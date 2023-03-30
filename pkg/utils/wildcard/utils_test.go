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

func TestCheckPatterns(t *testing.T) {
	var patterns []string
	var res bool
	patterns = []string{"*"}
	res = CheckPatterns(patterns, "default")
	assert.Equal(t, true, res)

	patterns = []string{"*", "default"}
	res = CheckPatterns(patterns, "default")
	assert.Equal(t, true, res)

	patterns = []string{"default2", "default"}
	res = CheckPatterns(patterns, "default1")
	assert.Equal(t, false, res)

	patterns = []string{"d*"}
	res = CheckPatterns(patterns, "default")
	assert.Equal(t, true, res)

	patterns = []string{"d*"}
	res = CheckPatterns(patterns, "test")
	assert.Equal(t, false, res)

	patterns = []string{}
	res = CheckPatterns(patterns, "test")
	assert.Equal(t, false, res)
}

func Test_MatchPatterns(t *testing.T) {
	testcases := []struct {
		description   string
		inputPatterns []string
		inputNs       []string
		expString1    string
		expString2    string
		expBool       bool
	}{
		{
			description:   "tc1",
			inputPatterns: []string{"default*", "test*"},
			inputNs:       []string{"default", "default1"},
			expString1:    "default*",
			expString2:    "default",
			expBool:       true,
		},
		{
			description:   "tc2",
			inputPatterns: []string{"test*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "test*",
			expString2:    "test",
			expBool:       true,
		},
		{
			description:   "tc3",
			inputPatterns: []string{"*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "*",
			expString2:    "default1",
			expBool:       true,
		},
		{
			description:   "tc4",
			inputPatterns: []string{"a*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc5",
			inputPatterns: nil,
			inputNs:       []string{"default1", "test"},
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc6",
			inputPatterns: []string{"*"},
			inputNs:       nil,
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc7",
			inputPatterns: nil,
			inputNs:       nil,
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			str1, str2, actualBool := MatchPatterns(tc.inputPatterns, tc.inputNs...)
			assert.Equal(t, str1, tc.expString1)
			assert.Equal(t, str2, tc.expString2)
			assert.Equal(t, actualBool, tc.expBool)
		})
	}
}

func Test_SeperateWildcards(t *testing.T) {
	testcases := []struct {
		description string
		inputList   []string
		expList1    []string
		expList2    []string
	}{
		{
			description: "tc1",
			inputList:   []string{"test*", "default", "default1", "hello"},
			expList1:    []string{"test*"},
			expList2:    []string{"default", "default1", "hello"},
		},
		{
			description: "tc2",
			inputList:   []string{"test*", "default*", "default1?", "hello?"},
			expList1:    []string{"test*", "default*", "default1?", "hello?"},
			expList2:    nil,
		},
		{
			description: "tc3",
			inputList:   []string{"test", "default", "default1", "hello"},
			expList1:    nil,
			expList2:    []string{"test", "default", "default1", "hello"},
		},
		{
			description: "tc4",
			inputList:   nil,
			expList1:    nil,
			expList2:    nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			list1, list2 := SeperateWildcards(tc.inputList)
			assert.Equal(t, tc.expList1, list1)
			assert.Equal(t, tc.expList2, list2)
		})
	}
}
