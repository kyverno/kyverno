package variables

import (
	"reflect"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Attestors(t *testing.T) {
	tests := []struct {
		name       string
		attestors  []v1beta1.Attestor
		celOpts    []cel.EnvOption
		data       map[string]any
		wantResult []v1beta1.Attestor
		wantErr    bool
	}{
		{
			name: "standard",
			attestors: []v1beta1.Attestor{
				{
					Name: "notary",
					Notary: &v1beta1.Notary{
						Certs: &v1beta1.StringOrExpression{
							Expression: "data.foo[0]",
						},
						TSACerts: &v1beta1.StringOrExpression{
							Expression: "data.foo[1]",
						},
					},
				},
				{
					Name: "cosign-keyed",
					Cosign: &v1beta1.Cosign{
						Key: &v1beta1.Key{
							Expression: "data.foo[0]",
						},
					},
				},
				{
					Name: "cosign-cert",
					Cosign: &v1beta1.Cosign{
						Certificate: &v1beta1.Certificate{
							Certificate: &v1beta1.StringOrExpression{
								Expression: "data.foo[0]",
							},
							CertificateChain: &v1beta1.StringOrExpression{
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
			wantResult: []v1beta1.Attestor{
				{
					Name: "notary",
					Notary: &v1beta1.Notary{
						Certs: &v1beta1.StringOrExpression{
							Value:      "bar",
							Expression: "data.foo[0]",
						},
						TSACerts: &v1beta1.StringOrExpression{
							Value:      "baz",
							Expression: "data.foo[1]",
						},
					},
				},
				{
					Name: "cosign-keyed",
					Cosign: &v1beta1.Cosign{
						Key: &v1beta1.Key{
							Data:       "bar",
							Expression: "data.foo[0]",
						},
					},
				},
				{
					Name: "cosign-cert",
					Cosign: &v1beta1.Cosign{
						Certificate: &v1beta1.Certificate{
							Certificate: &v1beta1.StringOrExpression{
								Value:      "bar",
								Expression: "data.foo[0]",
							},
							CertificateChain: &v1beta1.StringOrExpression{
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
			// Example 1 (backward compat): static values pass through unchanged when no
			// Expression is set — matches the plain-string YAML form after UnmarshalJSON.
			name: "cosign-keyless-identity-static-values",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer:  &v1beta1.StringOrExpression{Value: "https://token.actions.githubusercontent.com"},
									Subject: &v1beta1.StringOrExpression{Value: "https://github.com/my-org/my-repo/.github/workflows/release.yml@refs/heads/main"},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{},
			data:    map[string]any{},
			wantResult: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer:  &v1beta1.StringOrExpression{Value: "https://token.actions.githubusercontent.com"},
									Subject: &v1beta1.StringOrExpression{Value: "https://github.com/my-org/my-repo/.github/workflows/release.yml@refs/heads/main"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// Example 2: derive subject from object.metadata.labels — one policy for all
			// services; the expected signer is encoded in the pod label "github-repo".
			name: "cosign-keyless-identity-subject-from-label",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									Subject: &v1beta1.StringOrExpression{
										Expression: `"https://github.com/" + object.metadata.labels["github-repo"] + "/.github/workflows/release.yml@refs/heads/main"`,
									},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("object", cel.DynType),
			},
			data: map[string]any{
				"object": map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"github-repo": "my-org/my-app",
						},
					},
				},
			},
			wantResult: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									Subject: &v1beta1.StringOrExpression{
										Value:      "https://github.com/my-org/my-app/.github/workflows/release.yml@refs/heads/main",
										Expression: `"https://github.com/" + object.metadata.labels["github-repo"] + "/.github/workflows/release.yml@refs/heads/main"`,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// Example 3: derive subjectRegExp from object.metadata.namespace — enforce a
			// per-namespace (per-team) signing convention.
			name: "cosign-keyless-identity-subjectregexp-from-namespace",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Expression: `"^https://github\\.com/my-org/" + object.metadata.namespace + "/.*"`,
									},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("object", cel.DynType),
			},
			data: map[string]any{
				"object": map[string]any{
					"metadata": map[string]any{
						"namespace": "team-a",
					},
				},
			},
			wantResult: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Value:      `^https://github\.com/my-org/team-a/.*`,
										Expression: `"^https://github\\.com/my-org/" + object.metadata.namespace + "/.*"`,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// Example 4: derive subjectRegExp from images.containers[0] — derive the
			// expected GitHub repo from the image reference (single-container pods).
			name: "cosign-keyless-identity-subjectregexp-from-image",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Expression: `"^https://github\\.com/" + images.containers[0].split("/")[1] + "/" + images.containers[0].split("/")[2].split(":")[0] + "/.*"`,
									},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("images", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
			},
			data: map[string]any{
				"images": map[string][]string{
					"containers": {"ghcr.io/my-org/my-app:v1.2.3"},
				},
			},
			wantResult: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value: "https://token.actions.githubusercontent.com",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Value:      `^https://github\.com/my-org/my-app/.*`,
										Expression: `"^https://github\\.com/" + images.containers[0].split("/")[1] + "/" + images.containers[0].split("/")[2].split(":")[0] + "/.*"`,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// All four identity fields evaluated via CEL expressions in a single identity.
			name: "cosign-keyless-all-four-identity-expressions",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Expression: "data.issuer",
									},
									Subject: &v1beta1.StringOrExpression{
										Expression: "data.subject",
									},
									IssuerRegExp: &v1beta1.StringOrExpression{
										Expression: "data.issuerRegExp",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Expression: "data.subjectRegExp",
									},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("data", cel.MapType(cel.StringType, cel.StringType)),
			},
			data: map[string]any{
				"data": map[string]string{
					"issuer":        "https://token.actions.githubusercontent.com",
					"subject":       "https://github.com/my-org/my-app/.github/workflows/release.yml@refs/heads/main",
					"issuerRegExp":  "https://token\\.actions\\.githubusercontent\\.com",
					"subjectRegExp": "^https://github\\.com/my-org/.*",
				},
			},
			wantResult: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Value:      "https://token.actions.githubusercontent.com",
										Expression: "data.issuer",
									},
									Subject: &v1beta1.StringOrExpression{
										Value:      "https://github.com/my-org/my-app/.github/workflows/release.yml@refs/heads/main",
										Expression: "data.subject",
									},
									IssuerRegExp: &v1beta1.StringOrExpression{
										Value:      `https://token\.actions\.githubusercontent\.com`,
										Expression: "data.issuerRegExp",
									},
									SubjectRegExp: &v1beta1.StringOrExpression{
										Value:      `^https://github\.com/my-org/.*`,
										Expression: "data.subjectRegExp",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			// Error: identity expression evaluates to a non-string type.
			name: "cosign-keyless-identity-expression-not-string",
			attestors: []v1beta1.Attestor{
				{
					Name: "cosign-keyless",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer: &v1beta1.StringOrExpression{
										Expression: "data.foo",
									},
								},
							},
						},
					},
				},
			},
			celOpts: []cel.EnvOption{
				cel.Variable("data", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
			},
			data: map[string]any{
				"data": map[string][]string{
					"foo": {"bar", "baz"},
				},
			},
			wantErr: true,
		},
		{
			name: "data not string",
			attestors: []v1beta1.Attestor{
				{
					Name: "notary",
					Notary: &v1beta1.Notary{
						Certs: &v1beta1.StringOrExpression{
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
