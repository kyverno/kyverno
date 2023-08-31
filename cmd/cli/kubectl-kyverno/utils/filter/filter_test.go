package filter

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
)

func Test_policy_Apply(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		result api.TestResults
		want   bool
	}{{
		name:   "empty result",
		value:  "test",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "empty value",
		value: "",
		result: api.TestResults{
			Policy: "test",
		},
		want: false,
	}, {
		name:   "empty value and result",
		value:  "",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "match",
		value: "test",
		result: api.TestResults{
			Policy: "test",
		},
		want: true,
	}, {
		name:  "no match",
		value: "test",
		result: api.TestResults{
			Policy: "not-test",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := policy{
				value: tt.value,
			}
			if got := f.Apply(tt.result); got != tt.want {
				t.Errorf("policy.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rule_Apply(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		result api.TestResults
		want   bool
	}{{
		name:   "empty result",
		value:  "test",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "empty value",
		value: "",
		result: api.TestResults{
			Rule: "test",
		},
		want: false,
	}, {
		name:   "empty value and result",
		value:  "",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "match",
		value: "test",
		result: api.TestResults{
			Rule: "test",
		},
		want: true,
	}, {
		name:  "no match",
		value: "test",
		result: api.TestResults{
			Rule: "not-test",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := rule{
				value: tt.value,
			}
			if got := f.Apply(tt.result); got != tt.want {
				t.Errorf("rule.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resource_Apply(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		result api.TestResults
		want   bool
	}{{
		name:   "empty result",
		value:  "test",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "empty value",
		value: "",
		result: api.TestResults{
			Resource: "test",
		},
		want: false,
	}, {
		name:   "empty value and result",
		value:  "",
		result: api.TestResults{},
		want:   true,
	}, {
		name:  "match",
		value: "test",
		result: api.TestResults{
			Resource: "test",
		},
		want: true,
	}, {
		name:  "no match",
		value: "test",
		result: api.TestResults{
			Resource: "not-test",
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := resource{
				value: tt.value,
			}
			if got := f.Apply(tt.result); got != tt.want {
				t.Errorf("resource.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_composite_Apply(t *testing.T) {
	tests := []struct {
		name    string
		filters []Filter
		result  api.TestResults
		want    bool
	}{
		{
			name:    "nil",
			filters: nil,
			result:  api.TestResults{},
			want:    true,
		}, {
			name:    "empty",
			filters: []Filter{},
			result:  api.TestResults{},
			want:    true,
		}, {
			name:    "policy match",
			filters: []Filter{policy{"test"}},
			result: api.TestResults{
				Policy: "test",
			},
			want: true,
		}, {
			name:    "policy no match",
			filters: []Filter{policy{"test"}},
			result: api.TestResults{
				Policy: "not-test",
			},
			want: false,
		}, {
			name:    "policy and resource match",
			filters: []Filter{policy{"test"}, resource{"resource"}},
			result: api.TestResults{
				Policy:   "test",
				Resource: "resource",
			},
			want: true,
		}, {
			name:    "policy match and resource no match",
			filters: []Filter{policy{"test"}, resource{"resource"}},
			result: api.TestResults{
				Policy:   "test",
				Resource: "not-resource",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := composite{
				filters: tt.filters,
			}
			if got := f.Apply(tt.result); got != tt.want {
				t.Errorf("composite.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		filter Filter
		errors []error
	}{
		{
			name:   "empty",
			in:     "",
			filter: composite{},
			errors: nil,
		}, {
			name:   "invalid key",
			in:     "foo=bar",
			filter: composite{},
			errors: []error{
				errors.New("Invalid test-case-selector (foo=bar). Parameter can only be policy, rule or resource."),
			},
		}, {
			name:   "invalid arg",
			in:     "policy",
			filter: composite{},
			errors: []error{
				errors.New("Invalid test-case-selector argument (policy). Parameter must be in the form `<key>=<value>`."),
			},
		}, {
			name:   "policy",
			in:     "policy=test",
			filter: composite{[]Filter{policy{"test"}}},
			errors: nil,
		}, {
			name:   "rule",
			in:     "rule=test",
			filter: composite{[]Filter{rule{"test"}}},
			errors: nil,
		}, {
			name:   "resource",
			in:     "resource=test",
			filter: composite{[]Filter{resource{"test"}}},
			errors: nil,
		}, {
			name:   "policy, rule and resource",
			in:     "policy=test,rule=test,resource=test",
			filter: composite{[]Filter{policy{"test"}, rule{"test"}, resource{"test"}}},
			errors: nil,
		}, {
			name:   "policy, rule, resource and errors",
			in:     "policy=test,rule=test,foo=bar,resource=test,policy",
			filter: composite{[]Filter{policy{"test"}, rule{"test"}, resource{"test"}}},
			errors: []error{
				errors.New("Invalid test-case-selector (foo=bar). Parameter can only be policy, rule or resource."),
				errors.New("Invalid test-case-selector argument (policy). Parameter must be in the form `<key>=<value>`."),
			},
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ParseFilter(tt.in)
			if !reflect.DeepEqual(got, tt.filter) {
				t.Errorf("ParseFilter() got = %v, want %v", got, tt.filter)
			}
			if !reflect.DeepEqual(got1, tt.errors) {
				t.Errorf("ParseFilter() got1 = %v, want %v", got1, tt.errors)
			}
		})
	}
}
