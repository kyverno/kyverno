package variables

import (
	"reflect"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name         string
		fs           billy.Filesystem
		resourcePath string
		path         string
		vals         *v1alpha1.ValuesSpec
		vars         []string
		want         *Variables
		wantErr      bool
	}{{
		name:         "empty",
		fs:           nil,
		resourcePath: "",
		path:         "",
		vals:         nil,
		vars:         nil,
		want:         &Variables{},
		wantErr:      false,
	}, {
		name:         "vars",
		fs:           nil,
		resourcePath: "",
		path:         "",
		vals:         nil,
		vars: []string{
			"foo=bar",
		},
		want: &Variables{
			variables: map[string]string{
				"foo": "bar",
			},
		},
		wantErr: false,
	}, {
		name:         "values",
		fs:           nil,
		resourcePath: "",
		path:         "",
		vals: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"bar": "baz",
			},
		},
		vars: nil,
		want: &Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		wantErr: false,
	}, {
		name:         "values and vars",
		fs:           nil,
		resourcePath: "",
		path:         "",
		vals: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"bar": "baz",
			},
		},
		vars: []string{
			"foo=bar",
		},
		want: &Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"bar": "baz",
				},
			},
			variables: map[string]string{
				"foo": "bar",
			},
		},
		wantErr: false,
	}, {
		name:         "values file",
		fs:           nil,
		resourcePath: "",
		path:         "../_testdata/values/limit-configmap-for-sa.yaml",
		vals:         nil,
		vars:         nil,
		want: &Variables{
			values: &v1alpha1.ValuesSpec{
				NamespaceSelectors: []v1alpha1.NamespaceSelector{{
					Name: "test1",
					Labels: map[string]string{
						"foo.com/managed-state": "managed",
					},
				}},
				Policies: []v1alpha1.Policy{{
					Name: "limit-configmap-for-sa",
					Resources: []v1alpha1.Resource{{
						Name: "any-configmap-name-good",
						Values: map[string]interface{}{
							"request.operation": "UPDATE",
						},
					}, {
						Name: "any-configmap-name-bad",
						Values: map[string]interface{}{
							"request.operation": "UPDATE",
						},
					}},
				}},
			},
		},
		wantErr: false,
	}, {
		name:         "values file and vars",
		fs:           nil,
		resourcePath: "",
		path:         "../_testdata/values/limit-configmap-for-sa.yaml",
		vals:         nil,
		vars: []string{
			"foo=bar",
		},
		want: &Variables{
			values: &v1alpha1.ValuesSpec{
				NamespaceSelectors: []v1alpha1.NamespaceSelector{{
					Name: "test1",
					Labels: map[string]string{
						"foo.com/managed-state": "managed",
					},
				}},
				Policies: []v1alpha1.Policy{{
					Name: "limit-configmap-for-sa",
					Resources: []v1alpha1.Resource{{
						Name: "any-configmap-name-good",
						Values: map[string]interface{}{
							"request.operation": "UPDATE",
						},
					}, {
						Name: "any-configmap-name-bad",
						Values: map[string]interface{}{
							"request.operation": "UPDATE",
						},
					}},
				}},
			},
			variables: map[string]string{
				"foo": "bar",
			},
		},
		wantErr: false,
	}, {
		name:         "bad values file",
		fs:           nil,
		resourcePath: "",
		path:         "../_testdata/values/bad-format.yaml",
		vals:         nil,
		vars:         nil,
		want:         nil,
		wantErr:      true,
	}, {
		name:         "values and values file",
		fs:           nil,
		resourcePath: "",
		path:         "../_testdata/values/limit-configmap-for-sa.yaml",
		vals: &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"bar": "baz",
			},
		},
		vars: nil,
		want: &Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"bar": "baz",
				},
			},
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.fs, tt.resourcePath, tt.path, tt.vals, tt.vars...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}
