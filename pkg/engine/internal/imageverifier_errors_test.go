package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"net"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsInfrastructureErrorExcludesRegistryResponses pins that only an inability
// to reach the registry maps to Error. Error blocks solely under failurePolicy
// Fail, so classifying a registry HTTP response as Error would admit an image
// under failurePolicy Ignore that a Fail would have denied.
func TestIsInfrastructureErrorExcludesRegistryResponses(t *testing.T) {
	assert.False(t, isInfrastructureError(nil))

	netErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	assert.True(t, isInfrastructureError(netErr), "an unreachable registry is infrastructure")
	assert.True(t, isInfrastructureError(fmt.Errorf("wrapped: %w", netErr)), "wrapping must not lose the classification")

	for _, code := range []int{
		http.StatusUnauthorized, http.StatusForbidden,
		http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	} {
		err := &transport.Error{StatusCode: code}
		assert.False(t, isInfrastructureError(err), "registry response %d must fail closed, not degrade to Error", code)
	}

	assert.False(t, isInfrastructureError(errors.New("signature mismatch")))
}

// TestHasResourceDependentGate pins which entries must not be served from the
// cache. A hit returns a pass without running verifyImage, so any gate that
// reads the admission request would be skipped for every resource after the
// first.
func TestHasResourceDependentGate(t *testing.T) {
	tests := []struct {
		name     string
		entry    kyvernov1.ImageVerification
		siblings []kyvernov1.ImageVerification
		want     bool
	}{{
		name:  "no gate",
		entry: kyvernov1.ImageVerification{ImageReferences: []string{"*"}},
		want:  false,
	}, {
		name: "validate.deny",
		entry: kyvernov1.ImageVerification{
			Validation: kyvernov1.ValidateImageVerification{Deny: &kyvernov1.Deny{}},
		},
		want: true,
	}, {
		name: "validate.message only is not a gate",
		entry: kyvernov1.ImageVerification{
			Validation: kyvernov1.ValidateImageVerification{Message: "nope"},
		},
		want: false,
	}, {
		name: "attestation conditions",
		entry: kyvernov1.ImageVerification{
			Attestations: []kyvernov1.Attestation{{
				Type:       "https://slsa.dev/provenance/v0.2",
				Conditions: []kyvernov1.AnyAllConditions{{}},
			}},
		},
		want: true,
	}, {
		name: "conditions on a later attestation still count",
		entry: kyvernov1.ImageVerification{
			Attestations: []kyvernov1.Attestation{
				{Type: "a"},
				{Type: "b", Conditions: []kyvernov1.AnyAllConditions{{}}},
			},
		},
		want: true,
	}, {
		name: "attestations without conditions",
		entry: kyvernov1.ImageVerification{
			Attestations: []kyvernov1.Attestation{{Type: "a"}, {Type: "b"}},
		},
		want: false,
	}, {
		name:  "deny declared on a sibling entry still blocks caching",
		entry: kyvernov1.ImageVerification{ImageReferences: []string{"*"}},
		siblings: []kyvernov1.ImageVerification{{
			Validation: kyvernov1.ValidateImageVerification{Deny: &kyvernov1.Deny{}},
		}},
		want: true,
	}, {
		name:  "deny on an attestors-only sibling, which never reaches validate itself",
		entry: kyvernov1.ImageVerification{Attestations: []kyvernov1.Attestation{{Type: "a"}}},
		siblings: []kyvernov1.ImageVerification{{
			Attestors:  []kyvernov1.AttestorSet{{}},
			Validation: kyvernov1.ValidateImageVerification{Deny: &kyvernov1.Deny{}},
		}},
		want: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := kyvernov1.Rule{VerifyImages: append(tt.siblings, tt.entry)}
			assert.Equal(t, tt.want, hasResourceDependentGate(rule, tt.entry))
		})
	}
}

// TestSubstituteRuleVariablesWithoutDeny pins that an entry carrying no deny
// survives substitution. HasValidateImageVerification is rule-level and true
// for a message-only validate block, so treating it as "every entry has a
// deny" panicked on a shape the CRD accepts.
func TestSubstituteRuleVariablesWithoutDeny(t *testing.T) {
	rule := kyvernov1.Rule{
		Name: "verify",
		VerifyImages: []kyvernov1.ImageVerification{{
			ImageReferences: []string{"ghcr.io/example/*"},
			// validate present, deny absent: the shape that panicked.
			Validation: kyvernov1.ValidateImageVerification{Message: "nope"},
		}, {
			ImageReferences: []string{"ghcr.io/other/*"},
		}},
	}
	require.True(t, rule.HasValidateImageVerification(), "precondition: the rule-level flag is set")

	out, err := SubstituteRuleVariables(rule, enginecontext.NewContext(jmespath.New(config.NewDefaultConfiguration(false))), logr.Discard())
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Len(t, out.VerifyImages, 2)
}

// TestHasUnbindableKeyMaterial pins which entries must not be cached because
// the key binds only a reference to their verification material, not the
// material itself.
func TestHasUnbindableKeyMaterial(t *testing.T) {
	const inlinePEM = "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"

	set := func(keys *kyvernov1.StaticKeyAttestor) kyvernov1.AttestorSet {
		return kyvernov1.AttestorSet{Entries: []kyvernov1.Attestor{{Keys: keys}}}
	}

	tests := []struct {
		name  string
		entry kyvernov1.ImageVerification
		want  bool
	}{{
		name:  "inline PEM is fully bound",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{PublicKeys: inlinePEM})}},
		want:  false,
	}, {
		name:  "keys.secret names material",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "k", Namespace: "n"}})}},
		want:  true,
	}, {
		name:  "keys.kms names material",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{KMS: "awskms:///alias/signer"})}},
		want:  true,
	}, {
		name:  "publicKeys holding a k8s reference names material",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://ns/secret"})}},
		want:  true,
	}, {
		name:  "keyless attestor binds no static material",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{{Keyless: &kyvernov1.KeylessAttestor{Issuer: "https://example.com"}}}}}},
		want:  false,
	}, {
		name: "a reference on an attestation's own attestor still counts",
		entry: kyvernov1.ImageVerification{
			Attestations: []kyvernov1.Attestation{{
				Type:      "https://slsa.dev/provenance/v0.2",
				Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{KMS: "gcpkms://x"})},
			}},
		},
		want: true,
	}, {
		name:  "no attestors at all",
		entry: kyvernov1.ImageVerification{ImageReferences: []string{"*"}},
		want:  false,
	}, {
		name:  "a KMS alias containing the PEM header is still a reference",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{PublicKeys: "awskms:///alias/-----BEGIN PUBLIC KEY-----"})}},
		want:  true,
	}, {
		name:  "an inline key with leading whitespace is still inline",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{set(&kyvernov1.StaticKeyAttestor{PublicKeys: "\n  " + inlinePEM})}},
		want:  false,
	}, {
		name:  "a reference nested one level down still counts",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{nestedSet(t, &kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "k", Namespace: "n"}})}},
		want:  true,
	}, {
		name:  "an inline key nested one level down is still bound",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{nestedSet(t, &kyvernov1.StaticKeyAttestor{PublicKeys: inlinePEM})}},
		want:  false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasUnbindableKeyMaterial(tt.entry))
		})
	}
}

// nestedSet wraps keys in an attestor nested one level, the shape
// verifyAttestorSet recurses into at verify time.
func nestedSet(t *testing.T, keys *kyvernov1.StaticKeyAttestor) kyvernov1.AttestorSet {
	t.Helper()
	inner := kyvernov1.AttestorSet{Entries: []kyvernov1.Attestor{{Keys: keys}}}
	raw, err := json.Marshal(inner)
	require.NoError(t, err)
	return kyvernov1.AttestorSet{Entries: []kyvernov1.Attestor{{Attestor: &apiextv1.JSON{Raw: raw}}}}
}
