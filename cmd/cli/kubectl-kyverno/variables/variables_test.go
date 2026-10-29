package variables

import (
	"reflect"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/values"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestVariables_OldObject(t *testing.T) {
	base := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "my-pod",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app": "demo",
				},
			},
		},
	}

	t.Run("nil valuesSpec returns nil", func(t *testing.T) {
		v := Variables{}
		assert.Nil(t, v.OldObject(base, "my-pod"))
	})

	t.Run("empty valuesSpec returns nil", func(t *testing.T) {
		v := Variables{values: &v1alpha1.ValuesSpec{}}
		assert.Nil(t, v.OldObject(base, "my-pod"))
	})

	t.Run("globalValues with full object request.oldObject", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "old-pod",
						},
					},
				},
			},
		}
		got := v.OldObject(base, "my-pod")
		assert.NotNil(t, got)
		assert.Equal(t, "old-pod", got.GetName())
	})

	t.Run("globalValues with dot-notation overrides on base", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject.metadata.labels.existing-label": "foo",
				},
			},
		}
		got := v.OldObject(base, "my-pod")
		assert.NotNil(t, got)
		assert.Equal(t, "my-pod", got.GetName())
		labels := got.GetLabels()
		assert.Equal(t, "foo", labels["existing-label"])
		assert.Equal(t, "demo", labels["app"])
		assert.Equal(t, map[string]string{"app": "demo"}, base.GetLabels())
	})

	t.Run("canonical request.oldObject takes precedence over oldObject alias deterministically", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "alias-pod",
						},
					},
					"request.oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "canonical-pod",
						},
					},
				},
			},
		}
		got := v.OldObject(base, "my-pod")
		assert.NotNil(t, got)
		assert.Equal(t, "canonical-pod", got.GetName())
	})

	t.Run("policy-specific resource values takes precedence over globalValues", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "global-old-pod",
						},
					},
				},
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "Pod",
										"metadata": map[string]interface{}{
											"name": "policy-old-pod",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		// Matching policy sees policy-specific value
		gotPolicy := v.OldObject(base, "my-pod", "vpol-test")
		assert.NotNil(t, gotPolicy)
		assert.Equal(t, "policy-old-pod", gotPolicy.GetName())

		// Non-matching policy falls back to global value
		gotOther := v.OldObject(base, "my-pod", "other-policy")
		assert.NotNil(t, gotOther)
		assert.Equal(t, "global-old-pod", gotOther.GetName())
	})

	t.Run("resource matching by namespace/name", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "default/my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": "ns-old-pod",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		got := v.OldObject(base, "my-pod", "vpol-test")
		assert.NotNil(t, got)
		assert.Equal(t, "ns-old-pod", got.GetName())
	})

	t.Run("CLI variables override", func(t *testing.T) {
		v := Variables{
			variables: map[string]string{
				"request.oldObject.metadata.labels.cli-label": "bar",
			},
		}
		got := v.OldObject(base, "my-pod")
		assert.NotNil(t, got)
		assert.Equal(t, "bar", got.GetLabels()["cli-label"])
	})

	t.Run("distinct oldObject per policy for the same resource", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				Policies: []v1alpha1.Policy{
					{
						Name: "policy-alpha",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": "alpha-old-pod",
										},
									},
								},
							},
						},
					},
					{
						Name: "policy-beta",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"metadata": map[string]interface{}{
											"name": "beta-old-pod",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		gotAlpha := v.OldObject(base, "my-pod", "policy-alpha")
		assert.NotNil(t, gotAlpha)
		assert.Equal(t, "alpha-old-pod", gotAlpha.GetName())

		gotBeta := v.OldObject(base, "my-pod", "policy-beta")
		assert.NotNil(t, gotBeta)
		assert.Equal(t, "beta-old-pod", gotBeta.GetName())
	})

	t.Run("global full object with policy dot override and CLI variable override", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "global-pod",
							"labels": map[string]interface{}{
								"app": "web",
								"env": "prod",
							},
						},
					},
				},
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject.metadata.labels.env": "staging",
								},
							},
						},
					},
				},
			},
			variables: map[string]string{
				"request.oldObject.metadata.labels.tier": "backend",
			},
		}

		// Targeted policy gets global full object merged with policy-specific dot override and CLI dot override
		gotTargeted := v.OldObject(base, "my-pod", "vpol-test")
		assert.NotNil(t, gotTargeted)
		assert.Equal(t, "global-pod", gotTargeted.GetName())
		labelsTargeted := gotTargeted.GetLabels()
		assert.Equal(t, "web", labelsTargeted["app"])
		assert.Equal(t, "staging", labelsTargeted["env"])
		assert.Equal(t, "backend", labelsTargeted["tier"])

		// Other policy gets global full object without vpol-test's dot override, but with CLI override
		gotOther := v.OldObject(base, "my-pod", "other-policy")
		assert.NotNil(t, gotOther)
		assert.Equal(t, "global-pod", gotOther.GetName())
		labelsOther := gotOther.GetLabels()
		assert.Equal(t, "web", labelsOther["app"])
		assert.Equal(t, "prod", labelsOther["env"])
		assert.Equal(t, "backend", labelsOther["tier"])
	})
	t.Run("alias precedence between request.oldObject and oldObject", func(t *testing.T) {
		// global has request.oldObject, policy has oldObject alias
		v1 := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "global-request-pod",
						},
					},
				},
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"oldObject": map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "Pod",
										"metadata": map[string]interface{}{
											"name": "resource-alias-pod",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		got1 := v1.OldObject(base, "my-pod", "vpol-test")
		assert.NotNil(t, got1)
		assert.Equal(t, "resource-alias-pod", got1.GetName())

		// reverse: global has oldObject alias, policy has request.oldObject
		v2 := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"oldObject": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "global-alias-pod",
						},
					},
				},
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "Pod",
										"metadata": map[string]interface{}{
											"name": "resource-request-pod",
										},
									},
								},
							},
						},
					},
				},
			},
		}
		got2 := v2.OldObject(base, "my-pod", "vpol-test")
		assert.NotNil(t, got2)
		assert.Equal(t, "resource-request-pod", got2.GetName())
	})

	t.Run("global dotted override does not replace policy full object", func(t *testing.T) {
		v := Variables{
			values: &v1alpha1.ValuesSpec{
				GlobalValues: map[string]interface{}{
					"request.oldObject.metadata.labels.env": "prod",
				},
				Policies: []v1alpha1.Policy{
					{
						Name: "vpol-test",
						Resources: []v1alpha1.Resource{
							{
								Name: "my-pod",
								Values: map[string]interface{}{
									"request.oldObject": map[string]interface{}{
										"apiVersion": "v1",
										"kind":       "Pod",
										"metadata": map[string]interface{}{
											"name":   "policy-pod",
											"labels": map[string]interface{}{"env": "staging"},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		got := v.OldObject(nil, "my-pod", "vpol-test")
		assert.NotNil(t, got)
		assert.Equal(t, "policy-pod", got.GetName())
		assert.Equal(t, "staging", got.GetLabels()["env"])
	})
}
