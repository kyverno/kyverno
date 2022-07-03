package v1

import (
	"encoding/json"
	"fmt"
	"reflect"

	wildcard "github.com/kyverno/go-wildcard"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type ImageExtractorConfigs map[string][]ImageExtractorConfig

type ImageExtractorConfig struct {
	// Path is the path to the object containing the image field in a custom resource.
	// It should be slash-separated. Each slash-separated key must be a valid YAML key or a wildcard '*'.
	// Wildcard keys are expanded in case of arrays or objects.
	Path string `json:"path" yaml:"path"`
	// Value is an optional name of the field within 'path' that points to the image URI.
	// This is useful when a custom 'key' is also defined.
	// +optional
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	// Name is the entry the image will be available under 'images.<name>' in the context.
	// If this field is not defined, image entries will appear under 'images.custom'.
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// Key is an optional name of the field within 'path' that will be used to uniquely identify an image.
	// Note - this field MUST be unique.
	// +optional
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
}

// Rule defines a validation, mutation, or generation control for matching resources.
// Each rules contains a match declaration to select resources, and an optional exclude
// declaration to specify which resources to exclude.
type Rule struct {
	// Name is a label to identify the rule, It must be unique within the policy.
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// MatchResources defines when this policy rule should be applied. The match
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the user name or role.
	// At least one kind is required.
	MatchResourcesXXX *MatchResources `json:"match,omitempty" yaml:"match,omitempty"`

	// ExcludeResources defines when this policy rule should not be applied. The exclude
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the name or role.
	// +optional
	ExcludeResourcesXXX *MatchResources `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	// ImageExtractors defines a mapping from kinds to ImageExtractorConfigs.
	// This config is only valid for verifyImages rules.
	// +optional
	ImageExtractors ImageExtractorConfigs `json:"imageExtractors,omitempty" yaml:"imageExtractors,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements. A direct list
	// of conditions (without `any` or `all` statements is supported for backwards compatibility but
	// will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	RawAnyAllConditions *apiextv1.JSON `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// Mutation is used to modify matching resources.
	// +optional
	Mutation Mutation `json:"mutate,omitempty" yaml:"mutate,omitempty"`

	// Validation is used to validate matching resources.
	// +optional
	Validation Validation `json:"validate,omitempty" yaml:"validate,omitempty"`

	// Generation is used to create new resources.
	// +optional
	Generation Generation `json:"generate,omitempty" yaml:"generate,omitempty"`

	// VerifyImages is used to verify image signatures and mutate them to add a digest
	// +optional
	VerifyImages []ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`
}

// HasMutate checks for mutate rule
func (r *Rule) HasMutate() bool {
	return !reflect.DeepEqual(r.Mutation, Mutation{})
}

// HasVerifyImages checks for verifyImages rule
func (r *Rule) HasVerifyImages() bool {
	return r.VerifyImages != nil && !reflect.DeepEqual(r.VerifyImages, ImageVerification{})
}

// HasImagesValidationChecks checks whether the verifyImages rule has validation checks
func (r *Rule) HasImagesValidationChecks() bool {
	for _, v := range r.VerifyImages {
		if v.VerifyDigest || v.Required {
			return true
		}
	}

	return false
}

// HasValidate checks for validate rule
func (r *Rule) HasValidate() bool {
	return !reflect.DeepEqual(r.Validation, Validation{})
}

// HasGenerate checks for generate rule
func (r *Rule) HasGenerate() bool {
	return !reflect.DeepEqual(r.Generation, Generation{})
}

// IsMutateExisting checks if the mutate rule applies to existing resources
func (r *Rule) IsMutateExisting() bool {
	return r.Mutation.Targets != nil
}

// IsCloneSyncGenerate checks if the generate rule has the clone block with sync=true
func (r *Rule) GetCloneSyncForGenerate() (clone bool, sync bool) {
	if !r.HasGenerate() {
		return
	}

	if r.Generation.Clone.Name != "" {
		clone = true
	}

	sync = r.Generation.Synchronize
	return
}

func (r *Rule) GetAnyAllConditions() apiextensions.JSON {
	return FromJSON(r.RawAnyAllConditions)
}

func (r *Rule) SetAnyAllConditions(in apiextensions.JSON) {
	r.RawAnyAllConditions = ToJSON(in)
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
	if r.MatchResourcesXXX == nil || r.ExcludeResourcesXXX == nil {
		return errs
	}
	if len(r.ExcludeResourcesXXX.All) > 0 || len(r.MatchResourcesXXX.All) > 0 {
		return errs
	}
	// if both have any then no resource should be common
	if len(r.MatchResourcesXXX.Any) > 0 && len(r.ExcludeResourcesXXX.Any) > 0 {
		for _, rmr := range r.MatchResourcesXXX.Any {
			for _, rer := range r.ExcludeResourcesXXX.Any {
				if reflect.DeepEqual(rmr, rer) {
					return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
				}
			}
		}
		return errs
	}
	excludeRoles := sets.NewString(r.ExcludeResourcesXXX.Roles...)
	excludeClusterRoles := sets.NewString(r.ExcludeResourcesXXX.ClusterRoles...)
	excludeKinds := sets.NewString(r.ExcludeResourcesXXX.Kinds...)
	excludeNamespaces := sets.NewString(r.ExcludeResourcesXXX.Namespaces...)
	excludeSubjects := sets.NewString()
	for _, subject := range r.ExcludeResourcesXXX.Subjects {
		subjectRaw, _ := json.Marshal(subject)
		excludeSubjects.Insert(string(subjectRaw))
	}
	excludeSelectorMatchExpressions := sets.NewString()
	if r.ExcludeResourcesXXX.Selector != nil {
		for _, matchExpression := range r.ExcludeResourcesXXX.Selector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	excludeNamespaceSelectorMatchExpressions := sets.NewString()
	if r.ExcludeResourcesXXX.NamespaceSelector != nil {
		for _, matchExpression := range r.ExcludeResourcesXXX.NamespaceSelector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeNamespaceSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	if len(excludeRoles) > 0 {
		if len(r.MatchResourcesXXX.Roles) == 0 || !excludeRoles.HasAll(r.MatchResourcesXXX.Roles...) {
			return errs
		}
	}
	if len(excludeClusterRoles) > 0 {
		if len(r.MatchResourcesXXX.ClusterRoles) == 0 || !excludeClusterRoles.HasAll(r.MatchResourcesXXX.ClusterRoles...) {
			return errs
		}
	}
	if len(excludeSubjects) > 0 {
		if len(r.MatchResourcesXXX.Subjects) == 0 {
			return errs
		}
		for _, subject := range r.MatchResourcesXXX.UserInfo.Subjects {
			subjectRaw, _ := json.Marshal(subject)
			if !excludeSubjects.Has(string(subjectRaw)) {
				return errs
			}
		}
	}
	if r.ExcludeResourcesXXX.Name != "" {
		if !wildcard.Match(r.ExcludeResourcesXXX.Name, r.MatchResourcesXXX.Name) {
			return errs
		}
	}
	if len(r.ExcludeResourcesXXX.Names) > 0 {
		excludeSlice := r.ExcludeResourcesXXX.Names
		matchSlice := r.MatchResourcesXXX.Names

		// if exclude block has something and match doesn't it means we
		// have a non empty set
		if len(r.MatchResourcesXXX.Names) == 0 {
			return errs
		}

		// if *any* name in match and exclude conflicts
		// we want user to fix that
		for _, matchName := range matchSlice {
			for _, excludeName := range excludeSlice {
				if wildcard.Match(excludeName, matchName) {
					return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
				}
			}
		}
		return errs
	}
	if len(excludeNamespaces) > 0 {
		if len(r.MatchResourcesXXX.Namespaces) == 0 || !excludeNamespaces.HasAll(r.MatchResourcesXXX.Namespaces...) {
			return errs
		}
	}
	if len(excludeKinds) > 0 {
		if len(r.MatchResourcesXXX.Kinds) == 0 || !excludeKinds.HasAll(r.MatchResourcesXXX.Kinds...) {
			return errs
		}
	}
	if r.MatchResourcesXXX.Selector != nil && r.ExcludeResourcesXXX.Selector != nil {
		if len(excludeSelectorMatchExpressions) > 0 {
			if len(r.MatchResourcesXXX.Selector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range r.MatchResourcesXXX.Selector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(r.ExcludeResourcesXXX.Selector.MatchLabels) > 0 {
			if len(r.MatchResourcesXXX.Selector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range r.MatchResourcesXXX.Selector.MatchLabels {
				if r.ExcludeResourcesXXX.Selector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if r.MatchResourcesXXX.NamespaceSelector != nil && r.ExcludeResourcesXXX.NamespaceSelector != nil {
		if len(excludeNamespaceSelectorMatchExpressions) > 0 {
			if len(r.MatchResourcesXXX.NamespaceSelector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range r.MatchResourcesXXX.NamespaceSelector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeNamespaceSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(r.ExcludeResourcesXXX.NamespaceSelector.MatchLabels) > 0 {
			if len(r.MatchResourcesXXX.NamespaceSelector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range r.MatchResourcesXXX.NamespaceSelector.MatchLabels {
				if r.ExcludeResourcesXXX.NamespaceSelector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if (r.MatchResourcesXXX.Selector == nil && r.ExcludeResourcesXXX.Selector != nil) ||
		(r.MatchResourcesXXX.Selector != nil && r.ExcludeResourcesXXX.Selector == nil) {
		return errs
	}
	if (r.MatchResourcesXXX.NamespaceSelector == nil && r.ExcludeResourcesXXX.NamespaceSelector != nil) ||
		(r.MatchResourcesXXX.NamespaceSelector != nil && r.ExcludeResourcesXXX.NamespaceSelector == nil) {
		return errs
	}
	if r.MatchResourcesXXX.Annotations != nil && r.ExcludeResourcesXXX.Annotations != nil {
		if !(reflect.DeepEqual(r.MatchResourcesXXX.Annotations, r.ExcludeResourcesXXX.Annotations)) {
			return errs
		}
	}
	if (r.MatchResourcesXXX.Annotations == nil && r.ExcludeResourcesXXX.Annotations != nil) ||
		(r.MatchResourcesXXX.Annotations != nil && r.ExcludeResourcesXXX.Annotations == nil) {
		return errs
	}
	return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
}

// Validate implements programmatic validation
func (r *Rule) Validate(path *field.Path, namespaced bool, clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, r.ValidateRuleType(path)...)
	errs = append(errs, r.ValidateMatchExcludeConflict(path)...)
	errs = append(errs, r.MatchResourcesXXX.Validate(path.Child("match"), namespaced, clusterResources)...)
	errs = append(errs, r.ExcludeResourcesXXX.Validate(path.Child("exclude"), namespaced, clusterResources)...)
	return errs
}
