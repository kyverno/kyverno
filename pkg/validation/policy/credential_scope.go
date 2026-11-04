package policy

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validateNestedContextScope checks namespace boundaries independently of
// context syntax and match-kind discovery, which can produce admission warnings.
func validateNestedContextScope(rule kyvernov1.Rule, namespaced bool, policyNamespace string) error {
	if !namespaced {
		return nil
	}
	if rule.Mutation != nil {
		if err := validateMutationContextScope(rule.Mutation.ForEachMutation, field.NewPath("mutate", "foreach"), policyNamespace); err != nil {
			return err
		}
		for i, target := range rule.Mutation.Targets {
			if err := validateContextScope(target.Context, field.NewPath("mutate", "targets").Index(i).Child("context"), policyNamespace); err != nil {
				return err
			}
		}
	}
	if rule.Validation != nil {
		if err := validateValidationContextScope(rule.Validation.ForEachValidation, field.NewPath("validate", "foreach"), policyNamespace); err != nil {
			return err
		}
	}
	if rule.Generation != nil {
		for i, item := range rule.Generation.ForEachGeneration {
			if err := validateContextScope(item.Context, field.NewPath("generate", "foreach").Index(i).Child("context"), policyNamespace); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMutationContextScope(items []kyvernov1.ForEachMutation, path *field.Path, policyNamespace string) error {
	for i, item := range items {
		itemPath := path.Index(i)
		if err := validateContextScope(item.Context, itemPath.Child("context"), policyNamespace); err != nil {
			return err
		}
		if err := validateMutationContextScope(item.GetForEachMutation(), itemPath.Child("foreach"), policyNamespace); err != nil {
			return err
		}
	}
	return nil
}

func validateValidationContextScope(items []kyvernov1.ForEachValidation, path *field.Path, policyNamespace string) error {
	for i, item := range items {
		itemPath := path.Index(i)
		if err := validateContextScope(item.Context, itemPath.Child("context"), policyNamespace); err != nil {
			return err
		}
		if err := validateValidationContextScope(item.GetForEachValidation(), itemPath.Child("foreach"), policyNamespace); err != nil {
			return err
		}
	}
	return nil
}

func validateContextScope(entries []kyvernov1.ContextEntry, path *field.Path, policyNamespace string) error {
	for i, entry := range entries {
		entryPath := path.Index(i)
		// Check each source independently so a malformed entry with multiple
		// sources cannot hide a forbidden reference behind another source.
		if entry.GlobalReference != nil {
			return fmt.Errorf("%s: globalReference is not allowed in namespaced policies", entryPath.Child("globalReference"))
		}
		if entry.ImageRegistry != nil {
			var secrets []string
			if credentials := entry.ImageRegistry.ImageRegistryCredentials; credentials != nil {
				secrets = credentials.Secrets
			}
			// Context credentials are literal references, including inside foreach.
			if _, err := kubeutils.ScopeSecretReferences(secrets, policyNamespace); err != nil {
				return fmt.Errorf("%s: %w", entryPath.Child("imageRegistry", "imageRegistryCredentials"), err)
			}
		}
	}
	return nil
}

// validateAttestorKeyScope checks static keys that a verifier can load from a
// Secret. Cosign and the manifest verifier read k8s://<namespace>/<name> with
// Kyverno's own credentials, so a namespaced policy must name only Secrets in
// its own namespace through keys.publicKeys, keys.secret or keys.kms.
func validateAttestorKeyScope(rule kyvernov1.Rule, namespaced bool, policyNamespace string) error {
	if !namespaced {
		return nil
	}
	for i, imageVerify := range rule.VerifyImages {
		path := field.NewPath("verifyImages").Index(i)
		// The deprecated key shorthand is converted into keys.publicKeys.
		if err := validateKeyReferenceScope(imageVerify.Key, policyNamespace); err != nil {
			return fmt.Errorf("%s: %w", path.Child("key"), err)
		}
		if err := validateAttestorSetsKeyScope(imageVerify.Attestors, path.Child("attestors"), policyNamespace); err != nil {
			return err
		}
		for j, attestation := range imageVerify.Attestations {
			if err := validateAttestorSetsKeyScope(attestation.Attestors, path.Child("attestations").Index(j).Child("attestors"), policyNamespace); err != nil {
				return err
			}
		}
	}
	if rule.Validation != nil && rule.Validation.Manifests != nil {
		if err := validateAttestorSetsKeyScope(rule.Validation.Manifests.Attestors, field.NewPath("validate", "manifests", "attestors"), policyNamespace); err != nil {
			return err
		}
	}
	return nil
}

func validateAttestorSetsKeyScope(sets []kyvernov1.AttestorSet, path *field.Path, policyNamespace string) error {
	for i := range sets {
		if err := validateAttestorSetKeyScope(sets[i], path.Index(i), policyNamespace); err != nil {
			return err
		}
	}
	return nil
}

func validateAttestorSetKeyScope(set kyvernov1.AttestorSet, path *field.Path, policyNamespace string) error {
	for i, entry := range set.Entries {
		entryPath := path.Child("entries").Index(i)
		// Check every key source, not only the one the verifier prefers, so a
		// later default change cannot expose a reference admitted here.
		if keys := entry.Keys; keys != nil {
			keysPath := entryPath.Child("keys")
			if err := validateKeyReferenceScope(keys.PublicKeys, policyNamespace); err != nil {
				return fmt.Errorf("%s: %w", keysPath.Child("publicKeys"), err)
			}
			if err := validateKeyReferenceScope(keys.KMS, policyNamespace); err != nil {
				return fmt.Errorf("%s: %w", keysPath.Child("kms"), err)
			}
			if keys.Secret != nil {
				if err := validateKeySecretScope(*keys.Secret, policyNamespace); err != nil {
					return fmt.Errorf("%s: %w", keysPath.Child("secret"), err)
				}
			}
		}
		if entry.Attestor != nil {
			attestorPath := entryPath.Child("attestor")
			// A nested set that cannot be decoded cannot be checked, so reject it
			// rather than admit keys this walk never saw.
			nested, err := kyvernov1.AttestorSetUnmarshal(entry.Attestor)
			if err != nil {
				return fmt.Errorf("%s: %w", attestorPath, err)
			}
			if err := validateAttestorSetKeyScope(*nested, attestorPath, policyNamespace); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateKeyReferenceScope rejects a k8s:// key with a static foreign
// namespace. Inline keys, KMS URIs and references whose scheme comes from a
// variable pass here; runtime scopes the substituted key before cosign reads it.
func validateKeyReferenceScope(keyRef, policyNamespace string) error {
	if !kubeutils.IsKeySecretReference(keyRef) {
		return nil
	}
	if variables.ReplaceAllVars(keyRef, func(string) string { return "" }) == keyRef {
		_, err := kubeutils.ScopeKeyReference(keyRef, policyNamespace)
		return err
	}
	ref := strings.TrimSpace(keyRef)[len(kubeutils.KeySecretReferencePrefix):]
	if err := validateImageRegistryCredentialScope([]string{ref}, policyNamespace); err != nil {
		return fmt.Errorf("key reference %q: %w", keyRef, err)
	}
	return nil
}

// validateKeySecretScope applies the runtime keys.secret rules: an empty
// namespace means the policy namespace, and a templated namespace is
// confined after substitution.
func validateKeySecretScope(secret kyvernov1.SecretReference, policyNamespace string) error {
	namespace := secret.Namespace
	if namespace == "" {
		namespace = policyNamespace
	}
	return validateImageRegistryCredentialScope([]string{namespace + "/" + secret.Name}, policyNamespace)
}
