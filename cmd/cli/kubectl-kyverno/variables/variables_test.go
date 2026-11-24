package variables

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
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
			var s store.Store
			v := Variables{
				values:    tt.values,
				variables: tt.variables,
			}
			v.SetInStore(&s)
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
			var s store.Store
			v := Variables{
				values:    tt.fields.values,
				variables: tt.fields.variables,
			}
			got, err := v.ComputeVariables(&s, tt.args.policy, tt.args.resource, tt.args.kind, tt.args.kindMap, tt.args.variables...)
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

func TestVariables_OldObjectOverride(t *testing.T) {
	globalOverride := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(1)}}
	perResourceOverrideA := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(2)}}
	perResourceOverrideB := map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}
	tests := []struct {
		name        string
		values      *v1alpha1.ValuesSpec
		policyNames []string
		resource    string
		want        map[string]interface{}
		wantErr     bool
	}{{
		name:     "nil values",
		values:   nil,
		resource: "deployment",
		want:     nil,
	}, {
		name:     "no override declared",
		values:   &v1alpha1.ValuesSpec{},
		resource: "deployment",
		want:     nil,
	}, {
		name: "global override only",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
		},
		resource: "deployment",
		want:     globalOverride,
	}, {
		name: "per-resource override takes precedence over global",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": perResourceOverrideA},
				}},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		want:        perResourceOverrideA,
	}, {
		name: "duplicate resource entries: the last one wins, as in 1.19.2",
		values: &v1alpha1.ValuesSpec{
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{
					{Name: "deployment", Values: map[string]interface{}{"request.oldObject": perResourceOverrideA}},
					{Name: "deployment", Values: map[string]interface{}{"request.oldObject": perResourceOverrideB}},
				},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		want:        perResourceOverrideB,
	}, {
		name: "per-resource override for an unrelated policy is ignored",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": perResourceOverrideA},
				}},
			}},
		},
		policyNames: []string{"pol-b"},
		resource:    "deployment",
		want:        globalOverride,
	}, {
		name: "last policy in declaration order wins when multiple policyNames declare an override",
		values: &v1alpha1.ValuesSpec{
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": perResourceOverrideA},
				}},
			}, {
				Name: "pol-b",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": perResourceOverrideB},
				}},
			}},
		},
		policyNames: []string{"pol-a", "pol-b"},
		resource:    "deployment",
		want:        perResourceOverrideB,
	}, {
		name: "per-resource override for a different resource name is ignored",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "other-deployment",
					Values: map[string]interface{}{"request.oldObject": perResourceOverrideA},
				}},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		want:        globalOverride,
	}, {
		name: "a non-map request.oldObject is an error, not a silent fallback to global",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": "not-a-map"},
				}},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		wantErr:     true,
	}, {
		name: "an explicit null request.oldObject is an error",
		values: &v1alpha1.ValuesSpec{
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject": nil},
				}},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		wantErr:     true,
	}, {
		name: "dotted keys fold into the nested override",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"request.oldObject.metadata.labels.x": "y",
				"request.oldObject.spec.replicas":     int64(4),
			},
		},
		resource: "deployment",
		want: map[string]interface{}{
			"metadata": map[string]interface{}{"labels": map[string]interface{}{"x": "y"}},
			"spec":     map[string]interface{}{"replicas": int64(4)},
		},
	}, {
		name: "a per-resource dotted key combines with, rather than replaces, the global map override",
		values: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{"request.oldObject": globalOverride},
			Policies: []v1alpha1.Policy{{
				Name: "pol-a",
				Resources: []v1alpha1.Resource{{
					Name:   "deployment",
					Values: map[string]interface{}{"request.oldObject.metadata.labels.x": "y"},
				}},
			}},
		},
		policyNames: []string{"pol-a"},
		resource:    "deployment",
		want: map[string]interface{}{
			"spec":     map[string]interface{}{"replicas": int64(1)},
			"metadata": map[string]interface{}{"labels": map[string]interface{}{"x": "y"}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Variables{values: tt.values}
			got, err := v.OldObjectOverride(tt.policyNames, tt.resource)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVariables_OldObjectOverrideDoesNotMutateValues(t *testing.T) {
	v := Variables{values: &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.oldObject": map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old"}}},
		},
		Policies: []v1alpha1.Policy{{
			Name: "pol-a",
			Resources: []v1alpha1.Resource{{
				Name:   "p1",
				Values: map[string]interface{}{"request.oldObject.metadata.labels.extra": "a-only"},
			}},
		}},
	}}
	got, err := v.OldObjectOverride([]string{"pol-a"}, "p1")
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old", "extra": "a-only"}}}, got)
	// pol-a's dotted key must not have been written into the global map.
	got, err = v.OldObjectOverride([]string{"pol-b"}, "p1")
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old"}}}, got)
}

func TestVariables_OldObjectOverrideQuotedDottedKey(t *testing.T) {
	v := Variables{values: &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{`request.oldObject.metadata.labels."app.example.com/tier"`: "old"},
	}}
	got, err := v.OldObjectOverride(nil, "p1")
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"app.example.com/tier": "old"}}}, got)
}

func TestVariables_OldObjectOverrideDottedMapNotMutated(t *testing.T) {
	v := Variables{values: &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.oldObject.metadata.labels": map[string]interface{}{"tier": "old"},
		},
		Policies: []v1alpha1.Policy{{
			Name: "pol-a",
			Resources: []v1alpha1.Resource{{
				Name:   "p1",
				Values: map[string]interface{}{"request.oldObject.metadata.labels.extra": "a-only"},
			}},
		}},
	}}
	_, err := v.OldObjectOverride([]string{"pol-a"}, "p1")
	assert.NoError(t, err)
	got, err := v.OldObjectOverride([]string{"pol-b"}, "p2")
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"tier": "old"}}}, got)
}
