package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/go-logr/logr"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// TestFirstInfrastructureErrorIsSetScoped pins the contract that makes the
// Error-vs-Fail decision safe: only the failing attestor set's own errors may
// be considered. errorList accumulates across every set of an attestation, so
// scanning all of it would let an earlier, satisfied set's network error turn
// a genuine signature rejection into an Error, which failurePolicy Ignore
// admits.
func TestFirstInfrastructureErrorIsSetScoped(t *testing.T) {
	infra := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	rejected := errors.New("signature mismatch")

	t.Run("reports an infrastructure error from this set", func(t *testing.T) {
		assert.Equal(t, infra, firstInfrastructureError([]error{rejected, infra}))
	})

	t.Run("a genuine rejection alone is not infrastructure", func(t *testing.T) {
		assert.Nil(t, firstInfrastructureError([]error{rejected}))
	})

	t.Run("an earlier set's error must not leak in", func(t *testing.T) {
		// errorList as a whole: set A hit the network error but was satisfied,
		// set B then rejected the signature.
		all := []error{infra, rejected}
		setBErrors := all[1:]
		assert.Nil(t, firstInfrastructureError(setBErrors), "set B rejected the signature; it must Fail, not Error")
		assert.Equal(t, infra, firstInfrastructureError(all), "scanning the whole list is the bug this guards against")
	})

	t.Run("no errors", func(t *testing.T) {
		assert.Nil(t, firstInfrastructureError(nil))
	})
}

// keyScopePolicyContext serves only what the key scope paths read; anything
// else panics on the nil embedded interface.
type keyScopePolicyContext struct {
	engineapi.PolicyContext
	policy kyvernov1.PolicyInterface
	jctx   enginecontext.Interface
}

func (c keyScopePolicyContext) Policy() kyvernov1.PolicyInterface    { return c.policy }
func (c keyScopePolicyContext) JSONContext() enginecontext.Interface { return c.jctx }

// recordingCache serves a pass for every cacheable lookup, so a test can show
// a cached verdict does not stand in for the key scope check.
type recordingCache struct {
	imageverifycache.Client
	gets int
}

func (c *recordingCache) IsEnabled() bool { return true }

func (c *recordingCache) Get(_ context.Context, _ metav1.Object, _, _ string, useCache bool) (bool, error) {
	c.gets++
	return useCache, nil
}

type recordingRegistryClient struct {
	engineapi.RegistryClient
	fetches int
}

func (c *recordingRegistryClient) FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error) {
	c.fetches++
	return nil, errors.New("registry must not be contacted")
}

func newKeyScopeVerifier(policy kyvernov1.PolicyInterface, ivCache imageverifycache.Client, rclient engineapi.RegistryClient) *imageVerifier {
	return &imageVerifier{
		logger:  logr.Discard(),
		rclient: rclient,
		ivCache: ivCache,
		policyContext: keyScopePolicyContext{
			policy: policy,
			jctx:   enginecontext.NewContext(jmespath.New(config.NewDefaultConfiguration(false))),
		},
		rule: kyvernov1.Rule{Name: "verify"},
		ivm:  &engineapi.ImageVerificationMetadata{},
	}
}

func tenantPolicy() *kyvernov1.Policy {
	return &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "tenant-a"}}
}

func keysSet(keys ...*kyvernov1.StaticKeyAttestor) kyvernov1.AttestorSet {
	set := kyvernov1.AttestorSet{}
	for _, k := range keys {
		set.Entries = append(set.Entries, kyvernov1.Attestor{Keys: k})
	}
	return set
}

// TestAttestorKeyRef pins how each key field reaches cosign. A namespaced
// Policy may only name Secrets in its own namespace, through any of the three
// fields cosign reads as k8s://; a ClusterPolicy keeps its reference as is.
func TestAttestorKeyRef(t *testing.T) {
	const inlinePEM = "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"
	tests := []struct {
		name      string
		namespace string
		keys      kyvernov1.StaticKeyAttestor
		want      string
		wantErr   bool
	}{
		{name: "namespaced: keys.secret in a foreign namespace", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "stolen-key", Namespace: "kyverno"}}, wantErr: true},
		{name: "namespaced: keys.secret without a namespace defaults to the policy namespace", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign"}}, want: "k8s://tenant-a/cosign"},
		{name: "namespaced: keys.secret in the policy namespace", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign", Namespace: "tenant-a"}}, want: "k8s://tenant-a/cosign"},
		{name: "namespaced: publicKeys naming a foreign Secret", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://kyverno/stolen-key"}, wantErr: true},
		{name: "namespaced: publicKeys naming a Secret in the policy namespace", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://tenant-a/cosign"}, want: "k8s://tenant-a/cosign"},
		{name: "namespaced: inline publicKeys", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{PublicKeys: inlinePEM}, want: inlinePEM},
		{name: "namespaced: kms naming a foreign Secret", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{KMS: "k8s://kyverno/stolen-key"}, wantErr: true},
		{name: "namespaced: kms uri", namespace: "tenant-a", keys: kyvernov1.StaticKeyAttestor{KMS: "awskms:///alias/signer"}, want: "awskms:///alias/signer"},
		{name: "cluster: keys.secret in any namespace", keys: kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign", Namespace: "kyverno"}}, want: "k8s://kyverno/cosign"},
		{name: "cluster: keys.secret without a namespace is formatted as before", keys: kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign"}}, want: "k8s:///cosign"},
		{name: "cluster: publicKeys reference", keys: kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://kyverno/cosign"}, want: "k8s://kyverno/cosign"},
		{name: "cluster: kms reference", keys: kyvernov1.StaticKeyAttestor{KMS: "k8s://kyverno/cosign"}, want: "k8s://kyverno/cosign"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attestorKeyRef(&tt.keys, tt.namespace)
			if tt.wantErr {
				var kse *keyScopeError
				require.ErrorAs(t, err, &kse)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestVerifyRejectsForeignAttestorKey pins that a namespaced Policy naming a
// Secret outside its namespace errors the rule before anything is looked up:
// no cache hit, no registry call, and so no verifier that could read it.
func TestVerifyRejectsForeignAttestorKey(t *testing.T) {
	const inlinePEM = "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----"
	foreignSecret := &kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "stolen-key", Namespace: "kyverno"}}

	tests := []struct {
		name  string
		entry kyvernov1.ImageVerification
	}{{
		name:  "keys.secret",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(foreignSecret)}},
	}, {
		name:  "publicKeys",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(&kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://kyverno/stolen-key"})}},
	}, {
		name:  "kms",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(&kyvernov1.StaticKeyAttestor{KMS: "k8s://kyverno/stolen-key"})}},
	}, {
		name:  "a reference split out of a publicKeys bundle",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(&kyvernov1.StaticKeyAttestor{PublicKeys: inlinePEM + "\nk8s://kyverno/stolen-key\n-----END PUBLIC KEY-----"})}},
	}, {
		// count 1 would pass on the inline key and never reach the second
		// entry, so only an up-front check sees it.
		name: "behind an entry that satisfies count",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{func() kyvernov1.AttestorSet {
			set := keysSet(&kyvernov1.StaticKeyAttestor{PublicKeys: inlinePEM}, foreignSecret)
			count := 1
			set.Count = &count
			return set
		}()}},
	}, {
		name:  "nested attestor set",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{nestedSet(t, foreignSecret)}},
	}, {
		// The verifier would use the inline key, but admission rejects any
		// populated foreign source, so a stored policy must fail the same way.
		name: "unused keys.secret beside an inline key",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(&kyvernov1.StaticKeyAttestor{
			PublicKeys: inlinePEM, Secret: &kyvernov1.SecretReference{Name: "stolen-key", Namespace: "kyverno"},
		})}},
	}, {
		name: "unused kms beside a local secret",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(&kyvernov1.StaticKeyAttestor{
			Secret: &kyvernov1.SecretReference{Name: "cosign", Namespace: "tenant-a"}, KMS: "k8s://kyverno/stolen-key",
		})}},
	}, {
		// verifyImage would skip this image and build no verifier, but the
		// validate handler reports the error here too, so the mutating path must.
		name:  "an image skipImageReferences would skip",
		entry: kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{keysSet(foreignSecret)}, SkipImageReferences: []string{"ghcr.io/example/app*"}},
	}, {
		name: "attestation attestor",
		entry: kyvernov1.ImageVerification{Attestations: []kyvernov1.Attestation{{
			Type:      "https://slsa.dev/provenance/v0.2",
			Attestors: []kyvernov1.AttestorSet{keysSet(foreignSecret)},
		}}},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &recordingCache{}
			rclient := &recordingRegistryClient{}
			iv := newKeyScopeVerifier(tenantPolicy(), cache, rclient)
			tt.entry.ImageReferences = []string{"ghcr.io/example/*"}
			tt.entry.UseCache = true
			info, err := imageutils.GetImageInfo("ghcr.io/example/app:v1", config.NewDefaultConfiguration(false))
			require.NoError(t, err)

			_, responses := iv.Verify(context.TODO(), tt.entry, []apiutils.ImageInfo{{ImageInfo: *info}}, config.NewDefaultConfiguration(false))

			require.Len(t, responses, 1)
			assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
			assert.Contains(t, responses[0].Message(), "kyverno")
			assert.Contains(t, responses[0].Message(), "invalid attestor key scope")
			assert.Zero(t, cache.gets, "a cached pass must not stand in for the scope check")
			assert.Zero(t, rclient.fetches, "nothing may be fetched for a rejected entry")
			assert.Equal(t, engineapi.ImageVerificationFail, iv.ivm.ImageVerificationStatus(info.String()))
		})
	}

	t.Run("an image the entry does not verify is skipped as before", func(t *testing.T) {
		iv := newKeyScopeVerifier(tenantPolicy(), &recordingCache{}, &recordingRegistryClient{})
		entry := kyvernov1.ImageVerification{
			ImageReferences: []string{"ghcr.io/example/*"},
			Attestors:       []kyvernov1.AttestorSet{keysSet(foreignSecret)},
		}
		info, err := imageutils.GetImageInfo("ghcr.io/other/app:v1", config.NewDefaultConfiguration(false))
		require.NoError(t, err)

		_, responses := iv.Verify(context.TODO(), entry, []apiutils.ImageInfo{{ImageInfo: *info}}, config.NewDefaultConfiguration(false))

		require.Len(t, responses, 1)
		assert.Equal(t, engineapi.RuleStatusSkip, responses[0].Status())
	})
}

// TestScopeAttestorKeysClusterPolicy pins that a ClusterPolicy is not
// confined: its references are checked by nobody here, as before.
func TestScopeAttestorKeysClusterPolicy(t *testing.T) {
	iv := newKeyScopeVerifier(&kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p"}}, nil, nil)
	entry := kyvernov1.ImageVerification{Attestors: []kyvernov1.AttestorSet{
		keysSet(&kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign", Namespace: "kyverno"}}),
		nestedSet(t, &kyvernov1.StaticKeyAttestor{PublicKeys: "k8s://kyverno/cosign"}),
	}}
	assert.NoError(t, iv.scopeAttestorKeys(entry))

	_, opts, _, err := iv.buildCosignVerifier(entry.Attestors[0].Entries[0], entry, "ghcr.io/example/app:v1", nil)
	require.NoError(t, err)
	assert.Equal(t, "k8s://kyverno/cosign", opts.Key)
}

// TestBuildCosignVerifierScopesKey pins the check where opts.Key is built, so
// a path that reaches the verifier without Verify's up-front check still
// cannot hand cosign a foreign reference.
func TestBuildCosignVerifierScopesKey(t *testing.T) {
	iv := newKeyScopeVerifier(tenantPolicy(), nil, nil)

	attestor := kyvernov1.Attestor{Keys: &kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "cosign"}}}
	_, opts, _, err := iv.buildCosignVerifier(attestor, kyvernov1.ImageVerification{}, "ghcr.io/example/app:v1", nil)
	require.NoError(t, err)
	assert.Equal(t, "k8s://tenant-a/cosign", opts.Key)

	attestor.Keys.Secret.Namespace = "kyverno"
	v, opts, _, err := iv.buildCosignVerifier(attestor, kyvernov1.ImageVerification{}, "ghcr.io/example/app:v1", nil)
	var kse *keyScopeError
	require.ErrorAs(t, err, &kse)
	assert.Nil(t, v)
	assert.Nil(t, opts)
}

// TestVerifyAttestorsForeignKeyIsRuleError pins that a scope error from
// buildCosignVerifier, nested or not, ends the set instead of counting as one
// attestor that failed, and reports Error like a credential scope error.
func TestVerifyAttestorsForeignKeyIsRuleError(t *testing.T) {
	iv := newKeyScopeVerifier(tenantPolicy(), nil, nil)
	foreign := &kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Name: "stolen-key", Namespace: "kyverno"}}
	info, err := imageutils.GetImageInfo("ghcr.io/example/app:v1", config.NewDefaultConfiguration(false))
	require.NoError(t, err)

	for name, set := range map[string]kyvernov1.AttestorSet{
		"direct": keysSet(foreign),
		"nested": nestedSet(t, foreign),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := iv.verifyAttestorSet(context.TODO(), set, kyvernov1.ImageVerification{}, apiutils.ImageInfo{ImageInfo: *info}, ".attestors[0]")
			var kse *keyScopeError
			require.ErrorAs(t, err, &kse)

			resp, _ := iv.verifyAttestors(context.TODO(), []kyvernov1.AttestorSet{set}, kyvernov1.ImageVerification{}, apiutils.ImageInfo{ImageInfo: *info})
			assert.Equal(t, engineapi.RuleStatusError, resp.Status())
		})
	}
}
