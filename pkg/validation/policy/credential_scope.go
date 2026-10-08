package policy

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
