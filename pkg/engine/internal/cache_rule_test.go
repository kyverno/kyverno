package internal

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/stretchr/testify/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_attestorCacheRule_BindsMaterialNotName(t *testing.T) {
	withKey := func(key string) *kyvernov1.ImageVerification {
		return &kyvernov1.ImageVerification{
			Attestors: []kyvernov1.AttestorSet{
				{
					Entries: []kyvernov1.Attestor{
						{
							Keys: &kyvernov1.StaticKeyAttestor{
								PublicKeys: key,
							},
						},
					},
				},
			},
		}
	}

	ruleA, err := attestorCacheRule("verify-rule", withKey("KEY-A"))
	assert.NoError(t, err)
	ruleB, err := attestorCacheRule("verify-rule", withKey("KEY-B"))
	assert.NoError(t, err)
	assert.NotEqual(t, ruleA, ruleB, "same rule name with different key material must produce different cache rules")

	// Positive control: identical material must produce identical cache rule
	ruleARepeat, err := attestorCacheRule("verify-rule", withKey("KEY-A"))
	assert.NoError(t, err)
	assert.Equal(t, ruleA, ruleARepeat, "identical rule and key material must produce identical cache rule")

	// Different rule names with same material must produce different cache rules
	ruleDifferentName, err := attestorCacheRule("other-rule", withKey("KEY-A"))
	assert.NoError(t, err)
	assert.NotEqual(t, ruleA, ruleDifferentName, "different rule names must produce different cache rules")

	// Nil image verification must return an error and not panic
	_, errNil := attestorCacheRule("verify-rule", nil)
	assert.Error(t, errNil)
}

func Test_hasUnbindableKeyMaterial(t *testing.T) {
	for _, tc := range []struct {
		name       string
		iv         kyvernov1.ImageVerification
		unbindable bool
	}{
		{
			name: "inline public key",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\n-----END PUBLIC KEY-----",
								},
							},
						},
					},
				},
			},
			unbindable: false,
		},
		{
			name: "kms key in attestor",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									KMS: "awskms:///alias/signing",
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "secret reference in attestor",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									Secret: &kyvernov1.SecretReference{
										Name:      "mysecret",
										Namespace: "default",
									},
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "nested attestor with kms key",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Attestor: &apiextv1.JSON{
									Raw: []byte(`{"entries":[{"keys":{"kms":"awskms:///alias/nested"}}]}`),
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "attestation with kms key",
			iv: kyvernov1.ImageVerification{
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "https://slsa.dev/provenance/v1",
						Attestors: []kyvernov1.AttestorSet{
							{
								Entries: []kyvernov1.Attestor{
									{
										Keys: &kyvernov1.StaticKeyAttestor{
											KMS: "gcpkms://...",
										},
									},
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "keyless certificates",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keyless: &kyvernov1.KeylessAttestor{
									Issuer:  "https://token.actions.githubusercontent.com",
									Subject: "https://github.com/kyverno/kyverno/.github/workflows/ci.yaml@refs/heads/main",
								},
							},
						},
					},
				},
			},
			unbindable: false,
		},
		{
			name: "k8s secret in publicKeys",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: "k8s://default/mysecret",
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "nested attestor with k8s secret in publicKeys",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Attestor: &apiextv1.JSON{
									Raw: []byte(`{"entries":[{"keys":{"publicKeys":"k8s://default/nested-secret"}}]}`),
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "attestation with k8s secret in publicKeys",
			iv: kyvernov1.ImageVerification{
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "https://slsa.dev/provenance/v1",
						Attestors: []kyvernov1.AttestorSet{
							{
								Entries: []kyvernov1.Attestor{
									{
										Keys: &kyvernov1.StaticKeyAttestor{
											PublicKeys: "k8s://default/att-secret",
										},
									},
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "attestation condition with k8s reference",
			iv: kyvernov1.ImageVerification{
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "https://slsa.dev/provenance/v1",
						Conditions: []kyvernov1.AnyAllConditions{
							{
								AllConditions: []kyvernov1.Condition{
									{
										RawKey:   &apiextv1.JSON{Raw: []byte(`"k8s://default/secret"`)},
										Operator: "Equals",
										RawValue: &apiextv1.JSON{Raw: []byte(`"val"`)},
									},
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name: "kms key in publicKeys containing PUBLIC KEY in name",
			iv: kyvernov1.ImageVerification{
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: "awskms:///alias/my-PUBLIC KEY-signing",
								},
							},
						},
					},
				},
			},
			unbindable: true,
		},
		{
			name:       "empty image verification",
			iv:         kyvernov1.ImageVerification{},
			unbindable: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.unbindable, hasUnbindableKeyMaterial(tc.iv))
		})
	}
}

func Test_attestorCacheRule_BindsConditionVariables(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	newCtx := func(builder string) enginecontext.Interface {
		ctx := enginecontext.NewContext(jp)
		_ = ctx.AddVariable("cm.data.builder", builder)
		return ctx
	}

	iv := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"{{ cm.data.builder }}"`)},
							},
						},
					},
				},
			},
		},
	}

	declared := map[string]bool{"cm": true}
	ruleA, err := attestorCacheRule("verify-rule", iv, newCtx("builder-A"), declared)
	assert.NoError(t, err)

	ruleB, err := attestorCacheRule("verify-rule", iv, newCtx("builder-B"), declared)
	assert.NoError(t, err)

	assert.NotEqual(t, ruleA, ruleB, "rotating ConfigMap value in condition must change cache rule")

	ruleARepeat, err := attestorCacheRule("verify-rule", iv, newCtx("builder-A"), declared)
	assert.NoError(t, err)
	assert.Equal(t, ruleA, ruleARepeat, "identical condition variable value must produce identical cache rule")
	assert.Equal(t, "{{ builder.id }}", substituteContextVariablesInString("{{ builder.id }}", newCtx("builder-A"), nil, declared))
}

func Test_attestorCacheRule_BindsDenyConditionVariables(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := enginecontext.NewContext(jp)
	_ = ctx.AddVariable("cm.data.allowed", "A")

	iv := &kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	iv.Validation.Deny.SetAnyAllConditions(map[string]interface{}{
		"all": []interface{}{map[string]interface{}{
			"key": "{{ cm.data.allowed }}", "operator": "Equals", "value": "x",
		}},
	})

	declared := map[string]bool{"cm": true}
	ruleA, err := attestorCacheRule("verify-rule", iv, ctx, declared)
	assert.NoError(t, err)

	rotated := enginecontext.NewContext(jp)
	_ = rotated.AddVariable("cm.data.allowed", "B")
	ruleB, err := attestorCacheRule("verify-rule", iv, rotated, declared)
	assert.NoError(t, err)
	assert.NotEqual(t, ruleA, ruleB, "rotating ConfigMap in deny conditions must produce different cache rule")
}

func Test_isStatementScopedVariable_ContextNameCollision(t *testing.T) {
	declared := map[string]bool{"builder": true, "cm": true}

	// 1. Unqualified predicate variable colliding with declared context entry name:
	// In EvaluateConditions, the attestation predicate takes precedence for builder.id,
	// so builder.id must remain statement-scoped (return true) even though "builder"
	// is a declared context entry.
	assert.True(t, isStatementScopedVariable("builder.id", nil, declared),
		"unqualified statement variable builder.id must remain statement-scoped despite context name collision")

	// 2. Qualified ConfigMap context variable:
	// cm.data.builder is an outer context reference and must not be statement-scoped (return false).
	assert.False(t, isStatementScopedVariable("cm.data.builder", nil, declared),
		"cm.data.builder must be identified as outer context")
}

func Test_attestorCacheRule_BindsAllSiblingDenyConditions(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := enginecontext.NewContext(jp)
	_ = ctx.AddVariable("cm.data.level", "warn")

	entry1 := kyvernov1.ImageVerification{
		ImageReferences: []string{"nginx*"},
	}
	entry2 := kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	entry2.Validation.Deny.SetAnyAllConditions(map[string]interface{}{
		"all": []interface{}{map[string]interface{}{
			"key": "static", "operator": "Equals", "value": "static",
		}},
	})
	entry3 := kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	entry3.Validation.Deny.SetAnyAllConditions(map[string]interface{}{
		"all": []interface{}{map[string]interface{}{
			"key": "{{ cm.data.level }}", "operator": "Equals", "value": "critical",
		}},
	})

	rule := kyvernov1.Rule{
		Name:         "multi-deny-rule",
		VerifyImages: []kyvernov1.ImageVerification{entry1, entry2, entry3},
	}

	declared := map[string]bool{"cm": true}
	ruleA, err := attestorCacheRule("multi-deny-rule", &entry1, ctx, declared, rule)
	assert.NoError(t, err)

	rotated := enginecontext.NewContext(jp)
	_ = rotated.AddVariable("cm.data.level", "fatal")
	ruleB, err := attestorCacheRule("multi-deny-rule", &entry1, rotated, declared, rule)
	assert.NoError(t, err)

	assert.NotEqual(t, ruleA, ruleB, "rotating ConfigMap in a later sibling deny condition must invalidate the cache rule for all entries")
}

func Test_attestorCacheRule_PreservesConditionType(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctxNum := enginecontext.NewContext(jp)
	_ = ctxNum.AddVariable("cm.data.val", 1)

	ctxStr := enginecontext.NewContext(jp)
	_ = ctxStr.AddVariable("cm.data.val", "1")

	iv := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ predicate.val }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"{{ cm.data.val }}"`)},
							},
						},
					},
				},
			},
		},
	}

	declared := map[string]bool{"cm": true}
	ruleNum, err := attestorCacheRule("verify-rule", iv, ctxNum, declared)
	assert.NoError(t, err)

	ruleStr, err := attestorCacheRule("verify-rule", iv, ctxStr, declared)
	assert.NoError(t, err)

	assert.NotEqual(t, ruleNum, ruleStr, "numeric 1 and string '1' must produce distinct cache rules")
}

func Test_hasUnresolvedConditionVariables(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := enginecontext.NewContext(jp)
	_ = ctx.AddVariable("cm.data.builder", "builder-A")

	// 1. Condition with unresolvable statement variable -> true
	unresolvedIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(unresolvedIV, ctx))

	// 2. Condition with all variables resolved by declared context -> false
	resolvedIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ cm.data.builder }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.False(t, hasUnresolvedConditionVariables(resolvedIV, ctx, map[string]bool{"cm": true}))
	assert.True(t, hasUnresolvedConditionVariables(resolvedIV, ctx), "without declared context entry, cm.data.builder must remain unresolved")

	// 2b. Condition with statement field containing .data. or request.* must not be treated as outer context
	ctxWithBuildData := enginecontext.NewContext(jp)
	_ = ctxWithBuildData.AddVariable("build.data.version", "v1.0.0")
	buildDataIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ build.data.version }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"v1.0.0"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(buildDataIV, ctxWithBuildData), "build.data.version must remain unresolved when build is not in declared context")

	ctxWithRequest := enginecontext.NewContext(jp)
	_ = ctxWithRequest.AddVariable("request.operation", "CREATE")
	requestIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ request.operation }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"CREATE"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(requestIV, ctxWithRequest), "request.operation must remain statement-scoped and unresolved when request is not in declared context")

	// 3. Attestation with no conditions -> false
	noCondIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
			},
		},
	}
	assert.False(t, hasUnresolvedConditionVariables(noCondIV, ctx))

	// 4. Deny condition with unresolvable variable -> true
	denyIV := &kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	denyIV.Validation.Deny.SetAnyAllConditions(map[string]interface{}{
		"all": []interface{}{map[string]interface{}{
			"key": "{{ scan.critical }}", "operator": "GreaterThan", "value": 0,
		}},
	})
	assert.True(t, hasUnresolvedConditionVariables(denyIV, ctx))

	// 5. Statement-scoped variables (predicate, @, named attestation) remain unresolved even if in context
	ctxWithStatement := enginecontext.NewContext(jp)
	_ = ctxWithStatement.AddVariable("predicate.builder.id", "builder-A")
	_ = ctxWithStatement.AddVariable("scan.critical", 0)

	predicateIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ predicate.builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(predicateIV, ctxWithStatement))

	namedAttIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				Name:          "scan",
				PredicateType: "https://example.com/scan",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ scan.critical }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`0`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(namedAttIV, ctxWithStatement))

	// 6. Unqualified statement-scoped variables (e.g. builder.id, scanner.uri) must remain statement-scoped
	// and unresolved even if a colliding variable exists in context and context entry is declared.
	ctxWithColliding := enginecontext.NewContext(jp)
	_ = ctxWithColliding.AddVariable("builder.id", "builder-A")
	unqualifiedIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(unqualifiedIV, ctxWithColliding, map[string]bool{"builder": true}), "unqualified builder.id must remain unresolved even if declared in context")

	// 7. Conditions containing $(...) reference substitutions must remain unresolved (un-cacheable)
	refIV := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				PredicateType: "https://example.com/scan",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"$(element.name)"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"test"`)},
							},
						},
					},
				},
			},
		},
	}
	assert.True(t, hasUnresolvedConditionVariables(refIV, ctx))

	denyRefIV := &kyvernov1.ImageVerification{
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}
	denyRefIV.Validation.Deny.SetAnyAllConditions(map[string]interface{}{
		"all": []interface{}{map[string]interface{}{
			"key": "$(element.name)", "operator": "Equals", "value": "test",
		}},
	})
	assert.True(t, hasUnresolvedConditionVariables(denyRefIV, ctx))
}

func Test_attestorCacheRule_StatementScopedVariablesNotBound(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctxA := enginecontext.NewContext(jp)
	_ = ctxA.AddVariable("predicate.builder.id", "builder-A")
	_ = ctxA.AddVariable("builder.id", "builder-A")
	_ = ctxA.AddVariable("scan.critical", 0)

	ctxB := enginecontext.NewContext(jp)
	_ = ctxB.AddVariable("predicate.builder.id", "builder-B")
	_ = ctxB.AddVariable("builder.id", "builder-B")
	_ = ctxB.AddVariable("scan.critical", 5)

	iv := &kyvernov1.ImageVerification{
		Attestations: []kyvernov1.Attestation{
			{
				Name:          "scan",
				PredicateType: "https://example.com/scan",
				Conditions: []kyvernov1.AnyAllConditions{
					{
						AllConditions: []kyvernov1.Condition{
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ predicate.builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ builder.id }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
							},
							{
								RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ scan.critical }}"`)},
								Operator: "Equals",
								RawValue: &apiextv1.JSON{Raw: []byte(`0`)},
							},
						},
					},
				},
			},
		},
	}

	ruleA, err := attestorCacheRule("verify-rule", iv, ctxA)
	assert.NoError(t, err)
	ruleB, err := attestorCacheRule("verify-rule", iv, ctxB)
	assert.NoError(t, err)

	assert.Equal(t, ruleA, ruleB, "statement-scoped variables must not be bound from context into cache rule")
}

var _ engineapi.RegistryClient = (*mockRegistryClient)(nil)

type mockRegistryClient struct {
	desc *gcrremote.Descriptor
}

func (m *mockRegistryClient) ForRef(ctx context.Context, ref string) (*engineapi.ImageData, error) {
	return &engineapi.ImageData{}, nil
}

func (m *mockRegistryClient) FetchImageDescriptor(ctx context.Context, ref string) (*gcrremote.Descriptor, error) {
	if m.desc != nil {
		return m.desc, nil
	}
	h, _ := gcrv1.NewHash("sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105")
	return &gcrremote.Descriptor{
		Descriptor: gcrv1.Descriptor{
			Digest: h,
		},
	}, nil
}

func (m *mockRegistryClient) Options(ctx context.Context) ([]gcrremote.Option, []name.Option, error) {
	return nil, nil, nil
}

func (m *mockRegistryClient) NameOptions() []name.Option {
	return nil
}

var _ imageverifycache.Client = (*recordingCacheClient)(nil)

type recordingCacheClient struct {
	getCalled   int
	setCalled   int
	lastGetRule string
	lastSetRule string
	returnHit   bool
}

func (r *recordingCacheClient) Get(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, error) {
	r.getCalled++
	r.lastGetRule = ruleName
	return r.returnHit, nil
}

func (r *recordingCacheClient) Set(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, error) {
	r.setCalled++
	r.lastSetRule = ruleName
	return true, nil
}

func (r *recordingCacheClient) GetWithPayload(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool) (bool, map[string][]byte, error) {
	r.getCalled++
	r.lastGetRule = ruleName
	return r.returnHit, nil, nil
}

func (r *recordingCacheClient) SetWithPayload(ctx context.Context, policy metav1.Object, ruleName string, imageRef string, useCache bool, payloads map[string][]byte) (bool, error) {
	r.setCalled++
	r.lastSetRule = ruleName
	return true, nil
}

func (r *recordingCacheClient) IsEnabled() bool {
	return true
}

func Test_ImageVerifier_CacheableGate(t *testing.T) {
	const inlineKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`

	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	res := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "test-pod", "namespace": "default"},
		},
	}
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}
	policyCtx, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)
	policyCtx = policyCtx.WithPolicy(policy)

	var imageInfo apiutils.ImageInfo
	imageInfo.Registry = "docker.io"
	imageInfo.Name = "nginx"
	imageInfo.Path = "library/nginx"
	imageInfo.Tag = "latest"
	imageInfo.Digest = "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"

	tests := []struct {
		name        string
		imageVerify kyvernov1.ImageVerification
		returnHit   bool
		wantGet     int
		wantSet     int
	}{
		{
			name: "KMS attestor: caching disabled, zero Get and Set calls",
			imageVerify: kyvernov1.ImageVerification{
				UseCache:        true,
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									KMS: "awskms:///arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
								},
							},
						},
					},
				},
			},
			wantGet: 0,
			wantSet: 0,
		},
		{
			name: "k8s:// Secret attestor: caching disabled, zero Get and Set calls",
			imageVerify: kyvernov1.ImageVerification{
				UseCache:        true,
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: "k8s://default/mysecret",
								},
							},
						},
					},
				},
			},
			wantGet: 0,
			wantSet: 0,
		},
		{
			name: "Attestation condition with $(...) reference: caching disabled, zero Get and Set calls",
			imageVerify: kyvernov1.ImageVerification{
				UseCache:        true,
				ImageReferences: []string{"*"},
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "https://example.com/scan",
						Conditions: []kyvernov1.AnyAllConditions{
							{
								AllConditions: []kyvernov1.Condition{
									{
										RawKey:   &apiextv1.JSON{Raw: []byte(`"$(element.name)"`)},
										Operator: "Equals",
										RawValue: &apiextv1.JSON{Raw: []byte(`"test"`)},
									},
								},
							},
						},
					},
				},
			},
			wantGet: 0,
			wantSet: 0,
		},
		{
			name: "Unresolved condition variable: caching disabled, zero Get and Set calls",
			imageVerify: kyvernov1.ImageVerification{
				UseCache:        true,
				ImageReferences: []string{"*"},
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "https://example.com/scan",
						Conditions: []kyvernov1.AnyAllConditions{
							{
								AllConditions: []kyvernov1.Condition{
									{
										RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ builder.id }}"`)},
										Operator: "Equals",
										RawValue: &apiextv1.JSON{Raw: []byte(`"builder-A"`)},
									},
								},
							},
						},
					},
				},
			},
			wantGet: 0,
			wantSet: 0,
		},
		{
			name: "Inline PEM key with cache hit: Get called with rule;sha256 format, zero Set calls",
			imageVerify: kyvernov1.ImageVerification{
				UseCache:        true,
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{
						Entries: []kyvernov1.Attestor{
							{
								Keys: &kyvernov1.StaticKeyAttestor{
									PublicKeys: inlineKey,
								},
							},
						},
					},
				},
			},
			returnHit: true,
			wantGet:   1,
			wantSet:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeCache := &recordingCacheClient{returnHit: tt.returnHit}
			iv := &imageVerifier{
				logger:        logr.Discard(),
				rclient:       &mockRegistryClient{},
				ivCache:       fakeCache,
				policyContext: policyCtx,
				rule:          kyvernov1.Rule{Name: "test-rule"},
				ivm:           &engineapi.ImageVerificationMetadata{},
			}

			_, responses := iv.verifyMatchedImage(context.Background(), tt.imageVerify, imageInfo, cfg)
			assert.Equal(t, tt.wantGet, fakeCache.getCalled, "Get calls mismatch")
			assert.Equal(t, tt.wantSet, fakeCache.setCalled, "Set calls mismatch")
			if tt.wantGet > 0 {
				assert.True(t, strings.HasPrefix(fakeCache.lastGetRule, "test-rule;"), "cache rule must start with rule name")
				assert.Equal(t, len(fakeCache.lastGetRule), len("test-rule;")+64, "cache rule must end with 64-hex sha256 hash")
				assert.NotEmpty(t, responses)
				assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status())
				assert.Contains(t, responses[0].Message(), "verified from cache")
			}
		})
	}
}

func Test_ImageVerifier_CacheHitRetainsValidation(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)

	res := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]any{"name": "test-pod", "namespace": "default"},
		},
	}
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
	}
	policyCtx, err := policycontext.NewPolicyContext(jp, res, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)
	policyCtx = policyCtx.WithPolicy(policy)

	var imageInfo apiutils.ImageInfo
	imageInfo.Registry = "docker.io"
	imageInfo.Name = "nginx"
	imageInfo.Path = "library/nginx"
	imageInfo.Tag = "latest"
	imageInfo.Digest = "sha256:b31bfb4d0213f254d361e0079deaaebefa4f82ba7aa76ef82e90b4935ad5b105"

	const inlineKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`

	ivEntry := kyvernov1.ImageVerification{
		UseCache:        true,
		ImageReferences: []string{"*"},
		Attestors: []kyvernov1.AttestorSet{
			{
				Entries: []kyvernov1.Attestor{
					{
						Keys: &kyvernov1.StaticKeyAttestor{
							PublicKeys: inlineKey,
						},
					},
				},
			},
		},
		Validation: kyvernov1.ValidateImageVerification{
			Deny: &kyvernov1.Deny{},
		},
	}

	// 1. Validation deny condition triggers: cache hit must return RuleFail
	ivEntryFail := *ivEntry.DeepCopy()
	ivEntryFail.Validation.Deny.SetAnyAllConditions(kyvernov1.AnyAllConditions{
		AllConditions: []kyvernov1.Condition{
			{
				RawKey:   &apiextv1.JSON{Raw: []byte(`"true"`)},
				Operator: kyvernov1.ConditionOperators["Equals"],
				RawValue: &apiextv1.JSON{Raw: []byte(`"true"`)},
			},
		},
	})
	ruleFail := kyvernov1.Rule{
		Name:         "test-rule",
		VerifyImages: []kyvernov1.ImageVerification{ivEntryFail},
	}
	fakeCache := &recordingCacheClient{returnHit: true}
	iv := &imageVerifier{
		logger:        logr.Discard(),
		rclient:       &mockRegistryClient{},
		ivCache:       fakeCache,
		policyContext: policyCtx,
		rule:          ruleFail,
		ivm:           &engineapi.ImageVerificationMetadata{},
	}
	_, responses := iv.verifyMatchedImage(context.Background(), ivEntryFail, imageInfo, cfg)
	assert.Equal(t, 1, fakeCache.getCalled)
	assert.NotEmpty(t, responses)
	assert.Equal(t, engineapi.RuleStatusFail, responses[0].Status())
	assert.Contains(t, responses[0].Message(), "validation in verifyImages failed")

	// 2. Validation deny condition does not trigger: cache hit returns RulePass
	ivEntryPass := *ivEntry.DeepCopy()
	ivEntryPass.Validation.Deny.SetAnyAllConditions(kyvernov1.AnyAllConditions{
		AllConditions: []kyvernov1.Condition{
			{
				RawKey:   &apiextv1.JSON{Raw: []byte(`"true"`)},
				Operator: kyvernov1.ConditionOperators["Equals"],
				RawValue: &apiextv1.JSON{Raw: []byte(`"false"`)},
			},
		},
	})
	rulePass := kyvernov1.Rule{
		Name:         "test-rule",
		VerifyImages: []kyvernov1.ImageVerification{ivEntryPass},
	}
	fakeCachePass := &recordingCacheClient{returnHit: true}
	ivPass := &imageVerifier{
		logger:        logr.Discard(),
		rclient:       &mockRegistryClient{},
		ivCache:       fakeCachePass,
		policyContext: policyCtx,
		rule:          rulePass,
		ivm:           &engineapi.ImageVerificationMetadata{},
	}
	_, responsesPass := ivPass.verifyMatchedImage(context.Background(), ivEntryPass, imageInfo, cfg)
	assert.Equal(t, 1, fakeCachePass.getCalled)
	assert.NotEmpty(t, responsesPass)
	assert.Equal(t, engineapi.RuleStatusPass, responsesPass[0].Status())
	assert.Contains(t, responsesPass[0].Message(), "verified from cache")
}
