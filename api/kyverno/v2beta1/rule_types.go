package v2beta1

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Rule defines a validation, mutation, or generation control for matching resources.
// Each rules contains a match declaration to select resources, and an optional exclude
// declaration to specify which resources to exclude.
type Rule struct {
	// Name is a label to identify the rule, It must be unique within the policy.
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name" yaml:"name"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []kyvernov1.ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// MatchResources defines when this policy rule should be applied. The match
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the user name or role.
	// At least one kind is required.
	MatchResources MatchResources `json:"match,omitempty" yaml:"match,omitempty"`

	// ExcludeResources defines when this policy rule should not be applied. The exclude
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the name or role.
	// +optional
	ExcludeResources MatchResources `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	// ImageExtractors defines a mapping from kinds to ImageExtractorConfigs.
	// This config is only valid for verifyImages rules.
	// +optional
	ImageExtractors kyvernov1.ImageExtractorConfigs `json:"imageExtractors,omitempty" yaml:"imageExtractors,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	RawAnyAllConditions *AnyAllConditions `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// CELPreconditions are used to determine if a policy rule should be applied by evaluating a
	// set of CEL conditions. It can only be used with the validate.cel subrule
	// +optional
	CELPreconditions []admissionregistrationv1.MatchCondition `json:"celPreconditions,omitempty" yaml:"celPreconditions,omitempty"`

	// Mutation is used to modify matching resources.
	// +optional
	Mutation kyvernov1.Mutation `json:"mutate,omitempty" yaml:"mutate,omitempty"`

	// Validation is used to validate matching resources.
	// +optional
	Validation Validation `json:"validate,omitempty" yaml:"validate,omitempty"`

	// Generation is used to create new resources.
	// +optional
	Generation kyvernov1.Generation `json:"generate,omitempty" yaml:"generate,omitempty"`

	// VerifyImages is used to verify image signatures and mutate them to add a digest
	// +optional
	VerifyImages []ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`

	// SkipBackgroundRequests bypasses admission requests that are sent by the background controller.
	// The default value is set to "true", it must be set to "false" to apply
	// generate and mutateExisting rules to those requests.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	SkipBackgroundRequests bool `json:"skipBackgroundRequests,omitempty" yaml:"skipBackgroundRequests,omitempty"`
}

// HasMutate checks for mutate rule
func (r *Rule) HasMutate() bool {
	return !datautils.DeepEqual(r.Mutation, kyvernov1.Mutation{})
}

// HasVerifyImages checks for verifyImages rule
func (r *Rule) HasVerifyImages() bool {
	for _, verifyImage := range r.VerifyImages {
		if !datautils.DeepEqual(verifyImage, ImageVerification{}) {
			return true
		}
	}
	return false
}

// HasVerifyImageChecks checks whether the verifyImages rule has validation checks
func (r *Rule) HasVerifyImageChecks() bool {
	for _, v := range r.VerifyImages {
		if v.VerifyDigest || v.Required {
			return true
		}
	}
	return false
}

// HasVerifyManifests checks for validate.manifests rule
func (r Rule) HasVerifyManifests() bool {
	return r.Validation.Manifests != nil && len(r.Validation.Manifests.Attestors) != 0
}

// HasValidatePodSecurity checks for validate.podSecurity rule
func (r Rule) HasValidatePodSecurity() bool {
	return r.Validation.PodSecurity != nil && !datautils.DeepEqual(r.Validation.PodSecurity, &kyvernov1.PodSecurity{})
}

// HasValidateCEL checks for validate.cel rule
func (r *Rule) HasValidateCEL() bool {
	return r.Validation.CEL != nil && !datautils.DeepEqual(r.Validation.CEL, &kyvernov1.CEL{})
}

// HasValidate checks for validate rule
func (r *Rule) HasValidate() bool {
	return !datautils.DeepEqual(r.Validation, Validation{})
}

// HasGenerate checks for generate rule
func (r *Rule) HasGenerate() bool {
	return !datautils.DeepEqual(r.Generation, kyvernov1.Generation{})
}

// IsMutateExisting checks if the mutate rule applies to existing resources
func (r *Rule) IsMutateExisting() bool {
	return r.Mutation.Targets != nil
}

func (r *Rule) GetGenerateTypeAndSync() (_ kyvernov1.GenerateType, sync bool) {
	if !r.HasGenerate() {
		return
	}
	return r.Generation.GetTypeAndSync()
}

// ValidateRuleType checks only one type of rule is defined per rule
func (r *Rule) ValidateRuleType(path *field.Path) (errs field.ErrorList) {
	ruleTypes := []bool{r.HasMutate(), r.HasValidate(), r.HasGenerate(), r.HasVerifyImages()}
	count := 0
	for _, v := range ruleTypes {
		if v {
			count++
		}
	}
	if count == 0 {
		errs = append(errs, field.Invalid(path, r, fmt.Sprintf("No operation defined in the rule '%s'.(supported operations: mutate,validate,generate,verifyImages)", r.Name)))
	} else if count != 1 {
		errs = append(errs, field.Invalid(path, r, fmt.Sprintf("Multiple operations defined in the rule '%s', only one operation (mutate,validate,generate,verifyImages) is allowed per rule", r.Name)))
	}

	if r.ImageExtractors != nil && !r.HasVerifyImages() {
		errs = append(errs, field.Invalid(path.Child("imageExtractors"), r, fmt.Sprintf("Invalid rule spec for rule '%s', imageExtractors can only be defined for verifyImages rule", r.Name)))
	}
	return errs
}

// ValidateMatchExcludeConflict checks if the resultant of match and exclude block is not an empty set
func (r *Rule) ValidateMatchExcludeConflict(path *field.Path) (errs field.ErrorList) {
	if len(r.ExcludeResources.All) > 0 || len(r.MatchResources.All) > 0 {
		return errs
	}
	// if both have any then no resource should be common
	if len(r.MatchResources.Any) > 0 && len(r.ExcludeResources.Any) > 0 {
		for _, rmr := range r.MatchResources.Any {
			for _, rer := range r.ExcludeResources.Any {
				if datautils.DeepEqual(rmr, rer) {
					return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
				}
			}
		}
		return errs
	}
	if datautils.DeepEqual(r.ExcludeResources.Any, r.MatchResources.Any) {
		return errs
	}
	if datautils.DeepEqual(r.ExcludeResources.All, r.MatchResources.All) {
		return errs
	}
	return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
}

func (r *Rule) ValidateGenerate(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	if !r.HasGenerate() {
		return nil
	}

	return r.Generation.Validate(path, namespaced, policyNamespace, clusterResources)
}

// Validate implements programmatic validation
func (r *Rule) Validate(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	errs = append(errs, r.ValidateRuleType(path)...)
	errs = append(errs, r.ValidateMatchExcludeConflict(path)...)
	errs = append(errs, r.MatchResources.Validate(path.Child("match"), namespaced, clusterResources)...)
	errs = append(errs, r.ExcludeResources.Validate(path.Child("exclude"), namespaced, clusterResources)...)
	errs = append(errs, r.ValidateGenerate(path, namespaced, policyNamespace, clusterResources)...)
	return errs
}
