package variables

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestVariables_Subresources(t *testing.T) {
	tests := []struct {
		name      string
		values    *v1alpha1.ValuesSpec
		variables map[string]string
		want      []v1alpha1.Subresource
	}{{
		name:      "nil values",
		values:    nil,
		variables: nil,
		want:      nil,
	}, {
		name: "nil subresources",
		values: &v1alpha1.ValuesSpec{
			Subresources: nil,
		},
		variables: nil,
		want:      nil,
	}, {
		name: "empty subresources",
		values: &v1alpha1.ValuesSpec{
			Subresources: []v1alpha1.Subresource{},
		},
		variables: nil,
		want:      nil,
	}, {
		name: "subresources",
		values: &v1alpha1.ValuesSpec{
			Subresources: []v1alpha1.Subresource{{}},
		},
		variables: nil,
		want:      []v1alpha1.Subresource{{}},
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
		values    *v1alpha1.ValuesSpec
		variables map[string]string
		want      map[string]Labels
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
		want:      nil,
	}, {
		name:      "empty",
		values:    &v1alpha1.ValuesSpec{},
		variables: nil,
		want:      nil,
	}, {
		name:      "values",
		values:    &vals.ValuesSpec,
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
	vals.ValuesSpec.Policies = append(vals.ValuesSpec.Policies, v1alpha1.Policy{
		Name: "limit-configmap-for-sa",
		Rules: []v1alpha1.Rule{{
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
		values    *v1alpha1.ValuesSpec
		variables map[string]string
	}{{
		name:      "nil",
		values:    nil,
		variables: nil,
	}, {
		name:      "empty",
		values:    &v1alpha1.ValuesSpec{},
		variables: nil,
	}, {
		name:      "values",
		values:    &vals.ValuesSpec,
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

func TestVariables_ComputeVariables(t *testing.T) {
	loadValues := func(path string) *v1alpha1.ValuesSpec {
		t.Helper()
		vals, err := values.Load(nil, path)
		assert.NoError(t, err)
		return &vals.ValuesSpec
	}
	type fields struct {
		values    *v1alpha1.ValuesSpec
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
