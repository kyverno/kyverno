package mutate

import (
	"context"
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// mockAuthChecker for testing - allows configuring auth responses
type mockAuthChecker struct {
	user    string
	canI    bool
	canIMsg string
	canIErr error
}

func (m *mockAuthChecker) User() string { return m.user }

func (m *mockAuthChecker) CanI(ctx context.Context, verbs []string, gvk, namespace, name, subresource string) (bool, string, error) {
	return m.canI, m.canIMsg, m.canIErr
}

func newMockAuth(user string, allowed bool, msg string, err error) *mockAuthChecker {
	return &mockAuthChecker{user: user, canI: allowed, canIMsg: msg, canIErr: err}
}

func TestValidate_MutualExclusivity(t *testing.T) {
	// sample patch for testing patchStrategicMerge
	samplePatch := []byte(`{"metadata": {"labels": {"env": "test"}}}`)

	tests := []struct {
		name        string
		rule        *kyvernov1.Rule
		wantErr     bool
		wantPath    string
		errContains string
	}{
		{
			name: "foreach only - should pass",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{
						List:            "items",
						PatchesJSON6902: "- op: add\n  path: /metadata/labels/foo\n  value: bar",
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "foreach + patchStrategicMerge - conflict",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{
						List:            "items",
						PatchesJSON6902: "- op: add\n  path: /metadata/labels/foo\n  value: bar",
					}},
					RawPatchStrategicMerge: &apiextv1.JSON{Raw: samplePatch},
				},
			},
			wantErr:     true,
			wantPath:    "foreach",
			errContains: "only one of",
		},
		{
			name: "foreach + top-level patchesJSON6902 - conflict",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{
						List:            "items",
						PatchesJSON6902: "- op: add\n  path: /metadata/labels/foo\n  value: bar",
					}},
					PatchesJSON6902: "- op: remove\n  path: /metadata/labels/baz",
				},
			},
			wantErr:     true,
			wantPath:    "foreach",
			errContains: "only one of",
		},
		{
			name: "patchStrategicMerge only - ok",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					RawPatchStrategicMerge: &apiextv1.JSON{Raw: samplePatch},
				},
			},
			wantErr: false,
		},
		{
			name: "patchesJSON6902 only - ok",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					PatchesJSON6902: "- op: add\n  path: /spec/replicas\n  value: 3",
				},
			},
			wantErr: false,
		},
		{
			name: "both patch types together - not allowed",
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					RawPatchStrategicMerge: &apiextv1.JSON{Raw: samplePatch},
					PatchesJSON6902:        "- op: add\n  path: /metadata/annotations/key\n  value: val",
				},
			},
			wantErr:     true,
			wantPath:    "foreach",
			errContains: "only one of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Mutate{
				rule:                  tc.rule,
				authCheckerBackground: newMockAuth("test-sa", true, "", nil),
				authCheckerReports:    newMockAuth("test-sa", true, "", nil),
			}
			_, path, err := m.Validate(context.Background(), nil)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tc.wantPath, path)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_CELPreconditions(t *testing.T) {
	// CEL preconditions should only work with validate.cel, not mutate
	t.Run("cel with mutate should fail", func(t *testing.T) {
		rule := &kyvernov1.Rule{
			Mutation: &kyvernov1.Mutation{
				PatchesJSON6902: "- op: add\n  path: /metadata/labels/test\n  value: test",
			},
			CELPreconditions: []admissionregistrationv1.MatchCondition{{
				Name:       "check-name",
				Expression: "object.metadata.name == 'test'",
			}},
		}
		m := &Mutate{
			rule:                  rule,
			authCheckerBackground: newMockAuth("sa", true, "", nil),
			authCheckerReports:    newMockAuth("sa", true, "", nil),
		}
		_, _, err := m.Validate(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "celPrecondition can only be used with validate.cel")
	})

	t.Run("mutate without cel - fine", func(t *testing.T) {
		rule := &kyvernov1.Rule{
			Mutation: &kyvernov1.Mutation{
				PatchesJSON6902: "- op: add\n  path: /metadata/labels/test\n  value: test",
			},
		}
		m := &Mutate{
			rule:                  rule,
			authCheckerBackground: newMockAuth("sa", true, "", nil),
			authCheckerReports:    newMockAuth("sa", true, "", nil),
		}
		_, _, err := m.Validate(context.Background(), nil)
		assert.NoError(t, err)
	})
}

func TestValidateAuth_Targets(t *testing.T) {
	makeTarget := func(apiVer, kind, ns, name string) kyvernov1.TargetResourceSpec {
		return kyvernov1.TargetResourceSpec{
			TargetSelector: kyvernov1.TargetSelector{
				ResourceSpec: kyvernov1.ResourceSpec{
					APIVersion: apiVer,
					Kind:       kind,
					Namespace:  ns,
					Name:       name,
				},
			},
		}
	}

	tests := []struct {
		name        string
		targets     []kyvernov1.TargetResourceSpec
		authOK      bool
		authMsg     string
		authErr     error
		wantErr     bool
		errContains string
	}{
		{
			name:    "single target - auth passes",
			targets: []kyvernov1.TargetResourceSpec{makeTarget("v1", "ConfigMap", "default", "my-cm")},
			authOK:  true,
			wantErr: false,
		},
		{
			name:        "single target - auth denied",
			targets:     []kyvernov1.TargetResourceSpec{makeTarget("v1", "Secret", "kube-system", "tls-cert")},
			authOK:      false,
			authMsg:     "cannot get/update secrets in kube-system",
			wantErr:     true,
			errContains: "auth check fails",
		},
		{
			name:        "auth check returns error",
			targets:     []kyvernov1.TargetResourceSpec{makeTarget("v1", "ConfigMap", "prod", "app-config")},
			authOK:      false,
			authErr:     errors.New("timeout talking to API server"),
			wantErr:     true,
			errContains: "timeout",
		},
		{
			name:    "variable in kind - skip auth",
			targets: []kyvernov1.TargetResourceSpec{makeTarget("v1", "{{request.object.kind}}", "default", "test")},
			authOK:  false, // would fail if checked
			authMsg: "would fail if actually checked",
			wantErr: false, // but we skip it
		},
		{
			name: "multiple targets all pass",
			targets: []kyvernov1.TargetResourceSpec{
				makeTarget("v1", "ConfigMap", "ns1", "cm1"),
				makeTarget("v1", "ConfigMap", "ns2", "cm2"),
			},
			authOK:  true,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Mutate{
				rule: &kyvernov1.Rule{
					Mutation: &kyvernov1.Mutation{
						PatchesJSON6902: "- op: add\n  path: /data/key\n  value: val",
						Targets:         tc.targets,
					},
				},
				authCheckerBackground: newMockAuth("system:serviceaccount:kyverno:bg-controller", tc.authOK, tc.authMsg, tc.authErr),
				authCheckerReports:    newMockAuth("reports-sa", true, "", nil),
			}
			_, _, err := m.Validate(context.Background(), nil)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAuthReports(t *testing.T) {
	tests := []struct {
		name         string
		kinds        []string
		authOK       bool
		authMsg      string
		authErr      error
		wantWarnings bool
		wantErr      bool
	}{
		{
			name:         "auth ok for pods",
			kinds:        []string{"Pod"},
			authOK:       true,
			wantWarnings: false,
		},
		{
			name:         "wildcard skips check",
			kinds:        []string{"*"},
			authOK:       false,
			authMsg:      "this won't be checked",
			wantWarnings: false,
		},
		{
			name:         "partial wildcard also skips",
			kinds:        []string{"Deployment*"},
			authOK:       false,
			wantWarnings: false,
		},
		{
			name:         "auth denied - returns warning not error",
			kinds:        []string{"Pod"},
			authOK:       false,
			authMsg:      "cannot list pods",
			wantWarnings: true,
			wantErr:      false,
		},
		{
			name:    "auth error propagates",
			kinds:   []string{"Pod"},
			authErr: errors.New("connection refused"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Mutate{
				rule: &kyvernov1.Rule{
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: tc.kinds},
					},
					Mutation: &kyvernov1.Mutation{
						PatchesJSON6902: "- op: add\n  path: /metadata/labels/managed\n  value: 'true'",
					},
				},
				authCheckerBackground: newMockAuth("bg-sa", true, "", nil),
				authCheckerReports:    newMockAuth("reports-sa", tc.authOK, tc.authMsg, tc.authErr),
			}
			warnings, _, err := m.Validate(context.Background(), nil)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tc.wantWarnings {
				assert.NotEmpty(t, warnings)
			} else {
				assert.Empty(t, warnings)
			}
		})
	}
}

func TestValidateForEach_Nested(t *testing.T) {
	t.Run("valid foreach", func(t *testing.T) {
		m := &Mutate{
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{
						List:            "spec.containers",
						PatchesJSON6902: "- op: add\n  path: /resources/limits/memory\n  value: 512Mi",
					}},
				},
			},
			authCheckerBackground: newMockAuth("sa", true, "", nil),
			authCheckerReports:    newMockAuth("sa", true, "", nil),
		}
		_, _, err := m.Validate(context.Background(), nil)
		assert.NoError(t, err)
	})

	t.Run("foreach missing both patch types", func(t *testing.T) {
		m := &Mutate{
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{List: "items"}},
				},
			},
			authCheckerBackground: newMockAuth("sa", true, "", nil),
			authCheckerReports:    newMockAuth("sa", true, "", nil),
		}
		_, _, err := m.Validate(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only one of")
	})

	t.Run("nested foreach with context - invalid", func(t *testing.T) {
		m := &Mutate{
			rule: &kyvernov1.Rule{
				Mutation: &kyvernov1.Mutation{
					ForEachMutation: []kyvernov1.ForEachMutation{{
						List: "items",
						ForEachMutation: &kyvernov1.ForEachMutationWrapper{
							Items: []kyvernov1.ForEachMutation{{
								List:            "nested",
								PatchesJSON6902: "- op: add\n  path: /foo\n  value: bar",
							}},
						},
						Context: []kyvernov1.ContextEntry{{Name: "somevar"}},
					}},
				},
			},
			authCheckerBackground: newMockAuth("sa", true, "", nil),
			authCheckerReports:    newMockAuth("sa", true, "", nil),
		}
		_, _, err := m.Validate(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nested foreach cannot contain other declarations")
	})
}

func TestNewMutateFactory(t *testing.T) {
	rule := &kyvernov1.Rule{Mutation: &kyvernov1.Mutation{}}

	t.Run("mock mode", func(t *testing.T) {
		m := NewMutateFactory(rule, nil, true, "bg-sa", "reports-sa")
		assert.NotNil(t, m)
		assert.NotNil(t, m.authCheckerBackground)
		assert.NotNil(t, m.authCheckerReports)
	})

	t.Run("empty background SA falls back to fake", func(t *testing.T) {
		m := NewMutateFactory(rule, nil, false, "", "reports-sa")
		assert.NotNil(t, m.authCheckerBackground)
	})

	t.Run("empty reports SA falls back to fake", func(t *testing.T) {
		m := NewMutateFactory(rule, nil, false, "bg-sa", "")
		assert.NotNil(t, m.authCheckerReports)
	})
}

func TestHelperMethods(t *testing.T) {
	t.Run("hasForEach", func(t *testing.T) {
		// with items
		m := &Mutate{rule: &kyvernov1.Rule{
			Mutation: &kyvernov1.Mutation{ForEachMutation: []kyvernov1.ForEachMutation{{List: "x"}}},
		}}
		assert.True(t, m.hasForEach())

		// empty slice
		m.rule.Mutation.ForEachMutation = []kyvernov1.ForEachMutation{}
		assert.False(t, m.hasForEach())

		// nil
		m.rule.Mutation.ForEachMutation = nil
		assert.False(t, m.hasForEach())
	})

	t.Run("hasPatchStrategicMerge", func(t *testing.T) {
		patch := []byte(`{"spec": {}}`)
		m := &Mutate{rule: &kyvernov1.Rule{
			Mutation: &kyvernov1.Mutation{RawPatchStrategicMerge: &apiextv1.JSON{Raw: patch}},
		}}
		assert.True(t, m.hasPatchStrategicMerge())

		m.rule.Mutation.RawPatchStrategicMerge = nil
		assert.False(t, m.hasPatchStrategicMerge())
	})

	t.Run("hasPatchesJSON6902", func(t *testing.T) {
		m := &Mutate{rule: &kyvernov1.Rule{
			Mutation: &kyvernov1.Mutation{PatchesJSON6902: "- op: add\n  path: /x\n  value: y"},
		}}
		assert.True(t, m.hasPatchesJSON6902())

		m.rule.Mutation.PatchesJSON6902 = ""
		assert.False(t, m.hasPatchesJSON6902())
	})
}
