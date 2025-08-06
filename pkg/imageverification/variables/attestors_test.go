package variables

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Attestors(t *testing.T) {
	tests := []struct {
		name       string
		attestors  []v1alpha1.Attestor
		celOpts    []cel.EnvOption
		data       map[string]any
		wantResult []v1alpha1.Attestor
		wantErr    bool
	}{
		{
			name: "standard",
			attestors: []v1alpha1.Attestor{
				{
					Name: "notary",
					Notary: &v1alpha1.Notary{
						Certs: &v1alpha1.StringOrExpression{
							Expression: "data.foo[0]",
						},
						TSACerts: &v1alpha1.StringOrExpression{
							Expression: "data.foo[1]",
						},
					},
				},
				{
					Name: "cosign-keyed",
					Cosign: &v1alpha1.Cosign{
						Key: &v1alpha1.Key{
							Expression: "data.foo[0]",
						},
					},
				},
				{
					Name: "cosign-cert",
					Cosign: &v1alpha1.Cosign{
						Certificate: &v1alpha1.Certificate{
							Certificate: &v1alpha1.StringOrExpression{
								Expression: "data.foo[0]",
							},
							CertificateChain: &v1alpha1.StringOrExpression{
								Expression: "data.foo[1]",
							},
						},
					},
				},
			},
			data: map[string]any{
				"data": map[string][]string{
					"foo": {
						"bar",
						"baz",
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("data", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
			},
			wantResult: []v1alpha1.Attestor{
				{
					Name: "notary",
					Notary: &v1alpha1.Notary{
						Certs: &v1alpha1.StringOrExpression{
							Value:      "bar",
							Expression: "data.foo[0]",
						},
						TSACerts: &v1alpha1.StringOrExpression{
							Value:      "baz",
							Expression: "data.foo[1]",
						},
					},
				},
				{
					Name: "cosign-keyed",
					Cosign: &v1alpha1.Cosign{
						Key: &v1alpha1.Key{
							Data:       "bar",
							Expression: "data.foo[0]",
						},
					},
				},
				{
					Name: "cosign-cert",
					Cosign: &v1alpha1.Cosign{
						Certificate: &v1alpha1.Certificate{
							Certificate: &v1alpha1.StringOrExpression{
								Value:      "bar",
								Expression: "data.foo[0]",
							},
							CertificateChain: &v1alpha1.StringOrExpression{
								Value:      "baz",
								Expression: "data.foo[1]",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "data not string",
			attestors: []v1alpha1.Attestor{
				{
					Name: "notary",
					Notary: &v1alpha1.Notary{
						Certs: &v1alpha1.StringOrExpression{
							Expression: "data.foo",
						},
					},
				},
			},
			data: map[string]any{
				"data": map[string][]string{
					"foo": {
						"bar",
						"baz",
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("data", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := cel.NewEnv(tt.celOpts...)
			assert.NoError(t, err)
			assert.NotNil(t, env)

			c, errList := CompileAttestors(field.NewPath("spec", "attestors"), tt.attestors, env)
			assert.Nil(t, errList)
			for i, att := range c {
				a, err := att.Evaluate(tt.data)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.True(t, reflect.DeepEqual(a, tt.wantResult[i]))
				}
			}
		})
	}
}
