package policy

import (
	"encoding/json"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate_VerifyImagesCredentialScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		cluster       bool
		noCredentials bool
		secret        string
		errorContains string
	}{
		{name: "bare secret", namespace: "tenant-a", secret: "pull-secret"},
		{name: "same namespace", namespace: "tenant-a", secret: "tenant-a/pull-secret"},
		{name: "foreign namespace", namespace: "tenant-a", secret: "tenant-b/pull-secret", errorContains: "instead of policy namespace"},
		{name: "installation namespace", namespace: "tenant-a", secret: "kyverno/pull-secret", errorContains: "instead of policy namespace"},
		{name: "malformed secret", namespace: "tenant-a", secret: "tenant-a/pull/extra", errorContains: "invalid name"},
		{name: "empty secret name", namespace: "tenant-a", secret: "tenant-a/", errorContains: "empty name"},
		{name: "malformed policy namespace", namespace: "tenant/a", secret: "pull-secret", errorContains: "policy namespace \"tenant/a\" is invalid"},
		{name: "missing policy namespace", secret: "pull-secret", errorContains: "policy namespace must not be empty"},
		{name: "missing namespace without credentials", noCredentials: true, errorContains: "policy namespace must not be empty"},
		{name: "bare secret variable", namespace: "tenant-a", secret: "{{ request.object.metadata.name }}"},
		{name: "same namespace secret variable", namespace: "tenant-a", secret: "tenant-a/{{ request.object.metadata.name }}"},
		{name: "namespace variable", namespace: "tenant-a", secret: "{{ request.object.metadata.namespace }}/pull-secret"},
		{name: "foreign namespace secret variable", namespace: "tenant-a", secret: "tenant-b/{{ request.object.metadata.name }}", errorContains: "instead of policy namespace"},
		{name: "cluster policy bare secret", cluster: true, secret: "pull-secret"},
		{name: "cluster policy foreign namespace", cluster: true, secret: "tenant-b/pull-secret"},
		{name: "no explicit credentials", namespace: "tenant-a", noCredentials: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var credentials *kyvernov1.ImageRegistryCredentials
			if !test.noCredentials {
				credentials = &kyvernov1.ImageRegistryCredentials{Secrets: []string{test.secret}}
			}
			spec := kyvernov1.Spec{Rules: []kyvernov1.Rule{{
				Name: "verify-image",
				MatchResources: kyvernov1.MatchResources{Any: []kyvernov1.ResourceFilter{{
					ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
				}}},
				VerifyImages: []kyvernov1.ImageVerification{{
					ImageReferences:          []string{"ghcr.io/verified/*"},
					Required:                 true,
					ImageRegistryCredentials: credentials,
				}},
			}}}
			var policy kyvernov1.PolicyInterface
			if test.cluster {
				policy = &kyvernov1.ClusterPolicy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image"},
					Spec:       spec,
				}
			} else {
				policy = &kyvernov1.Policy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image", Namespace: test.namespace},
					Spec:       spec,
				}
			}
			_, err := Validate(policy, nil, nil, true, "", "")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
				assert.Contains(t, err.Error(), "verifyImages[0].imageRegistryCredentials")
			} else {
				require.NoError(t, err)
			}
			if credentials != nil {
				assert.Equal(t, []string{test.secret}, credentials.Secrets, "admission validation must not rewrite the policy")
			}
		})
	}
}

func TestValidateImageRegistryCredentialScopeTemplates(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		secret        string
		errorContains string
	}{
		{name: "namespace variable", secret: "{{ request.namespace }}/pull-secret"},
		{name: "partial namespace variable", secret: "tenant-{{ request.object.metadata.labels.tenant }}/pull-secret"},
		{name: "namespace and name variables", secret: "{{ request.namespace }}/{{ request.object.metadata.name }}"},
		{name: "complete reference variable", secret: "{{ request.object.metadata.annotations.credential }}"},
		{name: "slash inside namespace expression", secret: `{{ request.object.metadata.labels."example.com/namespace" }}/pull-secret`},
		{name: "slash inside name expression", secret: `tenant-a/{{ request.object.metadata.labels."example.com/secret" }}`},
		{name: "foreign namespace with name variable", secret: "tenant-b/{{ request.object.metadata.name }}", errorContains: "instead of policy namespace"},
		{name: "foreign namespace with slash expression", secret: `tenant-b/{{ request.object.metadata.labels."example.com/secret" }}`, errorContains: "instead of policy namespace"},
		{name: "dynamic namespace with empty name", secret: "{{ request.namespace }}/", errorContains: "empty name"},
		{name: "dynamic namespace with invalid name", secret: "{{ request.namespace }}/INVALID", errorContains: "invalid name"},
		{name: "dynamic namespace with extra slash", secret: "{{ request.namespace }}/pull/secret", errorContains: "invalid name"},
		{name: "leading slash with namespace variable", secret: "/{{ request.namespace }}/pull-secret"},
		{name: "double leading slash remains invalid", secret: "//tenant-a/pull-secret", errorContains: "invalid name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateImageRegistryCredentialScope([]string{test.secret}, "tenant-a")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateImageRegistryCredentialScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		secrets       []string
		noCredentials bool
		errorContains string
	}{
		{name: "missing namespace and no credentials", noCredentials: true, errorContains: "policy namespace must not be empty"},
		{name: "missing namespace and empty credentials", errorContains: "policy namespace must not be empty"},
		{name: "malformed secret", namespace: "tenant-a", secrets: []string{"tenant-a/pull/extra"}, errorContains: "invalid name"},
		{name: "bare secret variable", namespace: "tenant-a", secrets: []string{"{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
		{name: "same namespace secret variable", namespace: "tenant-a", secrets: []string{"tenant-a/{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
		{name: "foreign namespace secret variable", namespace: "tenant-a", secrets: []string{"tenant-b/{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var credentials *kyvernov1.ImageRegistryCredentials
			if !test.noCredentials {
				credentials = &kyvernov1.ImageRegistryCredentials{Secrets: test.secrets}
			}
			entry := kyvernov1.ContextEntry{ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:                "ghcr.io/verified/app:latest",
				ImageRegistryCredentials: credentials,
			}}
			err := validateImageRegistry(entry, true, test.namespace)
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

const scopeTestPublicKey = "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM\n5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==\n-----END PUBLIC KEY-----"

func keySecretAttestor(namespace, name string) kyvernov1.Attestor {
	return kyvernov1.Attestor{Keys: &kyvernov1.StaticKeyAttestor{Secret: &kyvernov1.SecretReference{Namespace: namespace, Name: name}}}
}

func publicKeyAttestor(key string) kyvernov1.Attestor {
	return kyvernov1.Attestor{Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: key}}
}

func kmsAttestor(kms string) kyvernov1.Attestor {
	return kyvernov1.Attestor{Keys: &kyvernov1.StaticKeyAttestor{KMS: kms}}
}

func nestedAttestor(t *testing.T, entries ...kyvernov1.Attestor) kyvernov1.Attestor {
	t.Helper()
	raw, err := json.Marshal(kyvernov1.AttestorSet{Entries: entries})
	require.NoError(t, err)
	return kyvernov1.Attestor{Attestor: &apiextv1.JSON{Raw: raw}}
}

// attestorsVerification places the entry under test after an inline key so
// error paths prove the walk reports the offending entry index.
func attestorsVerification(entry kyvernov1.Attestor) kyvernov1.ImageVerification {
	return kyvernov1.ImageVerification{
		ImageReferences: []string{"ghcr.io/verified/*"},
		Required:        true,
		Attestors:       []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{publicKeyAttestor(scopeTestPublicKey), entry}}},
	}
}

func TestValidate_VerifyImagesAttestorKeyScope(t *testing.T) {
	t.Parallel()
	entriesPath := "verifyImages[0].attestors[0].entries[1]"
	tests := []struct {
		name          string
		cluster       bool
		verify        func(*testing.T) kyvernov1.ImageVerification
		errorContains string
		path          string
	}{
		{name: "secret same namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("tenant-a", "cosign-key"))
		}},
		{name: "secret default namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("", "cosign-key"))
		}},
		{name: "secret installation namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("kyverno", "stolen-key"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.secret"},
		{name: "secret foreign namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("tenant-b", "cosign-key"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.secret"},
		{name: "secret namespace variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("{{request.namespace}}", "cosign-key"))
		}},
		{name: "secret name variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("", "{{ request.object.metadata.name }}"))
		}},
		{name: "secret foreign namespace with name variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("kyverno", "{{ request.object.metadata.name }}"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.secret"},
		{name: "secret name with namespace separator", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("tenant-a", "kyverno/stolen-key"))
		}, errorContains: "invalid name", path: entriesPath + ".keys.secret"},
		{name: "public key same namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("k8s://tenant-a/cosign-key"))
		}},
		{name: "public key installation namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("k8s://kyverno/stolen-key"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.publicKeys"},
		{name: "public key mixed case scheme", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("K8S://kyverno/stolen-key"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.publicKeys"},
		{name: "public key surrounding space", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("  k8s://kyverno/stolen-key\n"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.publicKeys"},
		{name: "public key without namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("k8s://stolen-key"))
		}, errorContains: "must use the format", path: entriesPath + ".keys.publicKeys"},
		{name: "public key variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("{{ request.object.metadata.annotations.cosignkey }}"))
		}},
		{name: "public key namespace variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("k8s://{{request.namespace}}/cosign-key"))
		}},
		{name: "public key foreign namespace with name variable", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor("k8s://kyverno/{{ request.object.metadata.name }}"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.publicKeys"},
		{name: "inline public key", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(publicKeyAttestor(scopeTestPublicKey))
		}},
		{name: "kms installation namespace secret", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(kmsAttestor("k8s://kyverno/stolen-key"))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".keys.kms"},
		{name: "kms uri", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(kmsAttestor("awskms://kyverno/alias/cosign"))
		}},
		{name: "nested attestor foreign secret", verify: func(t *testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(nestedAttestor(t, keySecretAttestor("kyverno", "stolen-key")))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".attestor.entries[0].keys.secret"},
		{name: "doubly nested attestor foreign public key", verify: func(t *testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(nestedAttestor(t, publicKeyAttestor(scopeTestPublicKey), nestedAttestor(t, publicKeyAttestor("k8s://kyverno/stolen-key"))))
		}, errorContains: "instead of policy namespace", path: entriesPath + ".attestor.entries[1].attestor.entries[0].keys.publicKeys"},
		{name: "nested attestor same namespace secret", verify: func(t *testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(nestedAttestor(t, keySecretAttestor("tenant-a", "cosign-key")))
		}},
		{name: "malformed nested attestor", verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(kyvernov1.Attestor{Attestor: &apiextv1.JSON{Raw: []byte(`{"entries":"invalid"}`)}})
		}, errorContains: "failed to unmarshal attestor set", path: entriesPath + ".attestor"},
		{name: "attestation attestor foreign secret", verify: func(*testing.T) kyvernov1.ImageVerification {
			return kyvernov1.ImageVerification{
				ImageReferences: []string{"ghcr.io/verified/*"},
				Required:        true,
				Attestations: []kyvernov1.Attestation{{
					Type:      "https://example.com/attestation",
					Attestors: []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{keySecretAttestor("kyverno", "stolen-key")}}},
				}},
			}
		}, errorContains: "instead of policy namespace", path: "verifyImages[0].attestations[0].attestors[0].entries[0].keys.secret"},
		{name: "deprecated key installation namespace", verify: func(*testing.T) kyvernov1.ImageVerification {
			return kyvernov1.ImageVerification{ImageReferences: []string{"ghcr.io/verified/*"}, Required: true, Key: "k8s://kyverno/stolen-key"}
		}, errorContains: "instead of policy namespace", path: "verifyImages[0].key"},
		{name: "cluster policy foreign secret", cluster: true, verify: func(*testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(keySecretAttestor("kyverno", "cosign-key"))
		}},
		{name: "cluster policy foreign public key", cluster: true, verify: func(t *testing.T) kyvernov1.ImageVerification {
			return attestorsVerification(nestedAttestor(t, publicKeyAttestor("k8s://kyverno/cosign-key")))
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := kyvernov1.Spec{Rules: []kyvernov1.Rule{{
				Name: "verify-image",
				MatchResources: kyvernov1.MatchResources{Any: []kyvernov1.ResourceFilter{{
					ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
				}}},
				VerifyImages: []kyvernov1.ImageVerification{test.verify(t)},
			}}}
			var policy kyvernov1.PolicyInterface
			if test.cluster {
				policy = &kyvernov1.ClusterPolicy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image"},
					Spec:       spec,
				}
			} else {
				policy = &kyvernov1.Policy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image", Namespace: "tenant-a"},
					Spec:       spec,
				}
			}
			submitted := policy.GetSpec().DeepCopy()
			_, err := Validate(policy, nil, nil, true, "", "")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
				assert.Contains(t, err.Error(), "spec.rules[0]: "+test.path+": ")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, submitted, policy.GetSpec(), "admission validation must not rewrite the policy")
		})
	}
}

func TestValidateAttestorKeyScope_Manifests(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		entry         kyvernov1.Attestor
		errorContains string
	}{
		{name: "same namespace secret", entry: publicKeyAttestor("k8s://tenant-a/cosign-key")},
		{name: "inline key", entry: publicKeyAttestor(scopeTestPublicKey)},
		{name: "foreign secret", entry: publicKeyAttestor("k8s://kyverno/stolen-key"), errorContains: "validate.manifests.attestors[0].entries[0].keys.publicKeys: "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rule := kyvernov1.Rule{Validation: &kyvernov1.Validation{Manifests: &kyvernov1.Manifests{
				Attestors: []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{test.entry}}},
			}}}
			err := validateAttestorKeyScope(rule, true, "tenant-a")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, validateAttestorKeyScope(rule, false, ""), "cluster policies keep cluster-wide key references")
		})
	}
}

func TestValidateKeyReferenceScope(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name          string
		keyRef        string
		errorContains string
	}{
		{name: "empty"},
		{name: "inline key", keyRef: scopeTestPublicKey},
		{name: "kms uri", keyRef: "gcpkms://projects/p/locations/l/keyRings/r/cryptoKeys/k"},
		{name: "same namespace", keyRef: "k8s://tenant-a/cosign-key"},
		{name: "foreign namespace", keyRef: "k8s://kyverno/cosign-key", errorContains: "instead of policy namespace"},
		{name: "mixed case scheme", keyRef: "K8s://kyverno/cosign-key", errorContains: "instead of policy namespace"},
		{name: "empty namespace", keyRef: "k8s:///cosign-key", errorContains: "must use the format"},
		{name: "whole key variable", keyRef: "{{ request.object.metadata.annotations.key }}"},
		{name: "scheme from variable", keyRef: "{{ scheme }}kyverno/cosign-key"},
		{name: "namespace variable", keyRef: "k8s://{{ request.namespace }}/cosign-key"},
		{name: "partial namespace variable", keyRef: "k8s://tenant-{{ request.object.metadata.labels.tenant }}/cosign-key"},
		{name: "reference variable", keyRef: "k8s://{{ request.object.metadata.annotations.key }}"},
		{name: "foreign namespace with name variable", keyRef: "k8s://kyverno/{{ request.object.metadata.name }}", errorContains: "instead of policy namespace"},
		{name: "namespace variable with invalid name", keyRef: "k8s://{{ request.namespace }}/INVALID", errorContains: "invalid name"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateKeyReferenceScope(test.keyRef, "tenant-a")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
