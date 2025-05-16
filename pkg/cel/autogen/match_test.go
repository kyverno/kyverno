package autogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestCreateMatchConstraints(t *testing.T) {
	tests := []struct {
		name       string
		targets    []Target
		operations []admissionregistrationv1.OperationType
		want       *admissionregistrationv1.MatchResources
	}{{
		name:       "nil targets",
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want:       nil,
	}, {
		name:       "empty targets",
		targets:    []Target{},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want:       nil,
	}, {
		name: "nil operations",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		want: nil,
	}, {
		name: "empty operations",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{},
		want:       nil,
	}, {
		name: "single target",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}, {
		name: "multiple targets",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}, {
			Group:    "flop",
			Version:  "v1",
			Resource: "foos",
			Kind:     "Foo",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "foos",
			Kind:     "Foo",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "bars",
			Kind:     "Bar",
		}},
		operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		want: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v1"},
						Resources:   []string{"foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v2"},
						Resources:   []string{"bars", "foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateMatchConstraints(tt.targets, tt.operations)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateMatchConditions(t *testing.T) {
	tests := []struct {
		name         string
		replacements string
		targets      []Target
		conditions   []admissionregistrationv1.MatchCondition
		want         []admissionregistrationv1.MatchCondition
	}{{
		name: "nil targets",
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: nil,
	}, {
		name:    "empty targets",
		targets: []Target{},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: nil,
	}, {
		name: "nil conditions",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		want: nil,
	}, {
		name: "empty conditions",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{},
		want:       []admissionregistrationv1.MatchCondition{},
	}, {
		name: "single target",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-foo",
			Expression: "!((object.apiVersion == 'foo/v1' && object.kind =='Bar')) ? true : something",
		}},
	}, {
		name: "multiple targets",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}, {
			Group:    "flop",
			Version:  "v2",
			Resource: "foos",
			Kind:     "Foo",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-foo",
			Expression: "!((object.apiVersion == 'flop/v2' && object.kind =='Foo') || (object.apiVersion == 'foo/v1' && object.kind =='Bar')) ? true : something",
		}},
	}, {
		name:         "with name",
		replacements: "test",
		targets: []Target{{
			Group:    "foo",
			Version:  "v1",
			Resource: "bars",
			Kind:     "Bar",
		}},
		conditions: []admissionregistrationv1.MatchCondition{{
			Name:       "foo",
			Expression: "something",
		}},
		want: []admissionregistrationv1.MatchCondition{{
			Name:       "autogen-test-foo",
			Expression: "!((object.apiVersion == 'foo/v1' && object.kind =='Bar')) ? true : something",
		}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateMatchConditions(tt.replacements, tt.targets, tt.conditions)
			assert.Equal(t, tt.want, got)
		})
	}
}
