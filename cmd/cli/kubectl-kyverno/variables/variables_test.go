package variables

import (
	"reflect"
	"testing"

	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestVariables_HasVariables(t *testing.T) {
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		want      bool
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
		want:      false,
	}, {
		name:      "empty",
		values:    nil,
		variables: map[string]string{},
		want:      false,
	}, {
		name:   "not empty",
		values: nil,
		variables: map[string]string{
			"foo": "bar",
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.HasVariables(); got != tt.want {
				t.Errorf("Variables.HasVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariables_Subresources(t *testing.T) {
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		want      []valuesapi.Subresource
	}{{
		name:      "nil values",
		values:    nil,
		variables: nil,
		want:      nil,
	}, {
		name: "nil subresources",
		values: &valuesapi.Values{
			Subresources: nil,
		},
		variables: nil,
		want:      nil,
	}, {
		name: "empty subresources",
		values: &valuesapi.Values{
			Subresources: []valuesapi.Subresource{},
		},
		variables: nil,
		want:      nil,
	}, {
		name: "subresources",
		values: &valuesapi.Values{
			Subresources: []valuesapi.Subresource{{}},
		},
		variables: nil,
		want:      []valuesapi.Subresource{{}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.Subresources(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Variables.Subresources() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariables_NamespaceSelectors(t *testing.T) {
	vals, err := values.Load(nil, "../_testdata/values/limit-configmap-for-sa.yaml")
	assert.NoError(t, err)
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		want      map[string]Labels
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
		want:      nil,
	}, {
		name:      "empty",
		values:    &valuesapi.Values{},
		variables: nil,
		want:      nil,
	}, {
		name:      "values",
		values:    vals,
		variables: nil,
		want: map[string]map[string]string{
			"test1": {
				"foo.com/managed-state": "managed",
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.NamespaceSelectors(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Variables.NamespaceSelectors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariables_SetInStore(t *testing.T) {
	vals, err := values.Load(nil, "../_testdata/values/limit-configmap-for-sa.yaml")
	assert.NoError(t, err)
	vals.Policies = append(vals.Policies, valuesapi.Policy{
		Name: "limit-configmap-for-sa",
		Rules: []valuesapi.Rule{{
			Name: "rule",
			Values: map[string]interface{}{
				"foo": "bar",
			},
			ForeachValues: map[string][]interface{}{
				"baz": nil,
			},
		}},
	})
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
	}, {
		name:      "empty",
		values:    &valuesapi.Values{},
		variables: nil,
	}, {
		name:      "values",
		values:    vals,
		variables: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			v.SetInStore()
		})
	}
}

func TestVariables_HasPolicyVariables(t *testing.T) {
	vals, err := values.Load(nil, "../_testdata/values/limit-configmap-for-sa.yaml")
	assert.NoError(t, err)
	vals.Policies = append(vals.Policies, valuesapi.Policy{
		Name: "limit-configmap-for-sa",
		Rules: []valuesapi.Rule{{
			Name: "rule",
			Values: map[string]interface{}{
				"foo": "bar",
			},
			ForeachValues: map[string][]interface{}{
				"baz": nil,
			},
		}},
	})
	tests := []struct {
		name      string
		values    *valuesapi.Values
		variables map[string]string
		policy    string
		want      bool
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
		policy:    "test",
		want:      false,
	}, {
		name:      "empty",
		values:    &valuesapi.Values{},
		variables: nil,
		policy:    "test",
		want:      false,
	}, {
		name:      "values - test",
		values:    vals,
		variables: nil,
		policy:    "test",
		want:      false,
	}, {
		name:      "values - limit-configmap-for-sa",
		values:    vals,
		variables: nil,
		policy:    "limit-configmap-for-sa",
		want:      true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			if got := v.HasPolicyVariables(tt.policy); got != tt.want {
				t.Errorf("Variables.HasPolicyVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariables_ComputeVariables(t *testing.T) {
	loadValues := func(path string) *valuesapi.Values {
		t.Helper()
		vals, err := values.Load(nil, path)
		assert.NoError(t, err)
		return vals
	}
	type fields struct {
		values    *valuesapi.Values
		variables map[string]string
	}
	type args struct {
		policy    string
		resource  string
		kind      string
		kindMap   sets.Set[string]
		variables []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "nil",
			args: args{
				"limit-configmap-for-sa",
				"any-configmap-name-good",
				"ConfigMap",
				nil,
				nil,
			},
			want: map[string]interface{}{
				"request.operation": "CREATE",
			},
			wantErr: false,
		},
		{
			name: "values",
			fields: fields{
				loadValues("../_testdata/values/limit-configmap-for-sa.yaml"),
				nil,
			},
			args: args{
				"limit-configmap-for-sa",
				"any-configmap-name-good",
				"ConfigMap",
				nil,
				nil,
			},
			want: map[string]interface{}{
				"request.operation": "UPDATE",
			},
			wantErr: false,
		}, {
			name: "values",
			fields: fields{
				loadValues("../_testdata/values/limit-configmap-for-sa.yaml"),
				nil,
			},
			args: args{
				"test",
				"any-configmap-name-good",
				"ConfigMap",
				nil,
				nil,
			},
			want: map[string]interface{}{
				"request.operation": "CREATE",
			},
			wantErr: false,
		}, {
			name: "values",
			fields: fields{
				loadValues("../_testdata/values/global-values.yaml"),
				nil,
			},
			args: args{
				"test",
				"any-configmap-name-good",
				"ConfigMap",
				nil,
				nil,
			},
			want: map[string]interface{}{
				"baz":               "jee",
				"foo":               "bar",
				"request.operation": "CREATE",
			},
			wantErr: false,
		}, {
			name: "values and variables",
			fields: fields{
				loadValues("../_testdata/values/global-values.yaml"),
				map[string]string{
					"request.operation": "DELETE",
				},
			},
			args: args{
				"test",
				"any-configmap-name-good",
				"ConfigMap",
				nil,
				nil,
			},
			want: map[string]interface{}{
				"baz":               "jee",
				"foo":               "bar",
				"request.operation": "DELETE",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{
				values:    tt.fields.values,
				variables: tt.fields.variables,
			}
			got, err := v.ComputeVariables(tt.args.policy, tt.args.resource, tt.args.kind, tt.args.kindMap, tt.args.variables...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Variables.ComputeVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Variables.ComputeVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}
