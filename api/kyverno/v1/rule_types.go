package v1

import (
	"encoding/json"
	"fmt"

	"github.com/kyverno/kyverno/pkg/pss/utils"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
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
	// JMESPath is an optional JMESPath expression to apply to the image value.
	// This is useful when the extracted image begins with a prefix like 'docker://'.
	// The 'trim_prefix' function may be used to trim the prefix: trim_prefix(@, 'docker://').
	// Note - Image digest mutation may not be used when applying a JMESPAth to an image.
	// +optional
	JMESPath string `json:"jmesPath,omitempty" yaml:"jmesPath,omitempty"`
}

// Rule defines a validation, mutation, or generation control for matching resources.
// Each rules contains a match declaration to select resources, and an optional exclude
// declaration to specify which resources to exclude.
type Rule struct {
	// Name is a label to identify the rule, It must be unique within the policy.
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name" yaml:"name"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

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
	ImageExtractors ImageExtractorConfigs `json:"imageExtractors,omitempty" yaml:"imageExtractors,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements. A direct list
	// of conditions (without `any` or `all` statements is supported for backwards compatibility but
	// will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	RawAnyAllConditions *apiextv1.JSON `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// CELPreconditions are used to determine if a policy rule should be applied by evaluating a
	// set of CEL conditions. It can only be used with the validate.cel subrule
	// +optional
	CELPreconditions []admissionregistrationv1alpha1.MatchCondition `json:"celPreconditions,omitempty" yaml:"celPreconditions,omitempty"`

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

	// SkipBackgroundRequests bypasses admission requests that are sent by the background controller.
	// The default value is set to "true", it must be set to "false" to apply
	// generate and mutateExisting rules to those requests.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	SkipBackgroundRequests bool `json:"skipBackgroundRequests,omitempty" yaml:"skipBackgroundRequests,omitempty"`
}

// HasMutate checks for mutate rule
func (r *Rule) HasMutate() bool {
	return !datautils.DeepEqual(r.Mutation, Mutation{})
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
	for _, verifyImage := range r.VerifyImages {
		if verifyImage.VerifyDigest || verifyImage.Required {
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
	return r.Validation.PodSecurity != nil && !datautils.DeepEqual(r.Validation.PodSecurity, &PodSecurity{})
}

// HasValidateCEL checks for validate.cel rule
func (r *Rule) HasValidateCEL() bool {
	return r.Validation.CEL != nil && !datautils.DeepEqual(r.Validation.CEL, &CEL{})
}

// HasValidate checks for validate rule
func (r *Rule) HasValidate() bool {
	return !datautils.DeepEqual(r.Validation, Validation{})
}

// HasGenerate checks for generate rule
func (r *Rule) HasGenerate() bool {
	return !datautils.DeepEqual(r.Generation, Generation{})
}

// IsMutateExisting checks if the mutate rule applies to existing resources
func (r *Rule) IsMutateExisting() bool {
	return r.Mutation.Targets != nil
}

func (r *Rule) IsPodSecurity() bool {
	return r.Validation.PodSecurity != nil
}

func (r *Rule) GetGenerateTypeAndSync() (_ GenerateType, sync bool) {
	if !r.HasGenerate() {
		return
	}
	return r.Generation.GetTypeAndSync()
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
	if datautils.DeepEqual(r.ExcludeResources, MatchResources{}) {
		return errs
	}
	excludeRoles := sets.New(r.ExcludeResources.Roles...)
	excludeClusterRoles := sets.New(r.ExcludeResources.ClusterRoles...)
	excludeKinds := sets.New(r.ExcludeResources.Kinds...)
	excludeNamespaces := sets.New(r.ExcludeResources.Namespaces...)
	excludeSubjects := sets.New[string]()
	for _, subject := range r.ExcludeResources.Subjects {
		subjectRaw, _ := json.Marshal(subject)
		excludeSubjects.Insert(string(subjectRaw))
	}
	excludeSelectorMatchExpressions := sets.New[string]()
	if r.ExcludeResources.Selector != nil {
		for _, matchExpression := range r.ExcludeResources.Selector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	excludeNamespaceSelectorMatchExpressions := sets.New[string]()
	if r.ExcludeResources.NamespaceSelector != nil {
		for _, matchExpression := range r.ExcludeResources.NamespaceSelector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeNamespaceSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	if len(excludeRoles) > 0 {
		if len(r.MatchResources.Roles) == 0 || !excludeRoles.HasAll(r.MatchResources.Roles...) {
			return errs
		}
	}
	if len(excludeClusterRoles) > 0 {
		if len(r.MatchResources.ClusterRoles) == 0 || !excludeClusterRoles.HasAll(r.MatchResources.ClusterRoles...) {
			return errs
		}
	}
	if len(excludeSubjects) > 0 {
		if len(r.MatchResources.Subjects) == 0 {
			return errs
		}
		for _, subject := range r.MatchResources.UserInfo.Subjects {
			subjectRaw, _ := json.Marshal(subject)
			if !excludeSubjects.Has(string(subjectRaw)) {
				return errs
			}
		}
	}
	if r.ExcludeResources.Name != "" {
		if !wildcard.Match(r.ExcludeResources.Name, r.MatchResources.Name) {
			return errs
		}
	}
	if len(r.ExcludeResources.Names) > 0 {
		excludeSlice := r.ExcludeResources.Names
		matchSlice := r.MatchResources.Names

		// if exclude block has something and match doesn't it means we
		// have a non empty set
		if len(r.MatchResources.Names) == 0 {
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
		if len(r.MatchResources.Namespaces) == 0 || !excludeNamespaces.HasAll(r.MatchResources.Namespaces...) {
			return errs
		}
	}
	if len(excludeKinds) > 0 {
		if len(r.MatchResources.Kinds) == 0 || !excludeKinds.HasAll(r.MatchResources.Kinds...) {
			return errs
		}
	}
	if r.MatchResources.Selector != nil && r.ExcludeResources.Selector != nil {
		if len(excludeSelectorMatchExpressions) > 0 {
			if len(r.MatchResources.Selector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range r.MatchResources.Selector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(r.ExcludeResources.Selector.MatchLabels) > 0 {
			if len(r.MatchResources.Selector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range r.MatchResources.Selector.MatchLabels {
				if r.ExcludeResources.Selector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if r.MatchResources.NamespaceSelector != nil && r.ExcludeResources.NamespaceSelector != nil {
		if len(excludeNamespaceSelectorMatchExpressions) > 0 {
			if len(r.MatchResources.NamespaceSelector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range r.MatchResources.NamespaceSelector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeNamespaceSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(r.ExcludeResources.NamespaceSelector.MatchLabels) > 0 {
			if len(r.MatchResources.NamespaceSelector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range r.MatchResources.NamespaceSelector.MatchLabels {
				if r.ExcludeResources.NamespaceSelector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if (r.MatchResources.Selector == nil && r.ExcludeResources.Selector != nil) ||
		(r.MatchResources.Selector != nil && r.ExcludeResources.Selector == nil) {
		return errs
	}
	if (r.MatchResources.NamespaceSelector == nil && r.ExcludeResources.NamespaceSelector != nil) ||
		(r.MatchResources.NamespaceSelector != nil && r.ExcludeResources.NamespaceSelector == nil) {
		return errs
	}
	if r.MatchResources.Annotations != nil && r.ExcludeResources.Annotations != nil {
		if !datautils.DeepEqual(r.MatchResources.Annotations, r.ExcludeResources.Annotations) {
			return errs
		}
	}
	if (r.MatchResources.Annotations == nil && r.ExcludeResources.Annotations != nil) ||
		(r.MatchResources.Annotations != nil && r.ExcludeResources.Annotations == nil) {
		return errs
	}
	return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
}

// ValidateMutationRuleTargetNamespace checks if the targets are scoped to the policy's namespace
func (r *Rule) ValidateMutationRuleTargetNamespace(path *field.Path, namespaced bool, policyNamespace string) (errs field.ErrorList) {
	if r.HasMutate() && namespaced {
		for idx, target := range r.Mutation.Targets {
			if target.Namespace != "" && target.Namespace != policyNamespace {
				errs = append(errs, field.Invalid(path.Child("targets").Index(idx).Child("namespace"), target.Namespace, "This field can be ignored or should have value of the namespace where the policy is being created"))
			}
		}
	}
	return errs
}

func (r *Rule) ValidatePSaControlNames(path *field.Path) (errs field.ErrorList) {
	if r.IsPodSecurity() {
		podSecurity := r.Validation.PodSecurity
		forbiddenControls := []string{}
		if podSecurity.Level == "baseline" {
			forbiddenControls = utils.PSS_restricted_control_names
		}

		for idx, exclude := range podSecurity.Exclude {
			// container level control must specify images
			if containsString(utils.PSS_container_level_control, exclude.ControlName) {
				if len(exclude.Images) == 0 {
					errs = append(errs, field.Invalid(path.Child("podSecurity").Child("exclude").Index(idx).Child("controlName"), exclude.ControlName, "exclude.images must be specified for the container level control"))
				}
			} else if containsString(utils.PSS_pod_level_control, exclude.ControlName) {
				if len(exclude.Images) != 0 {
					errs = append(errs, field.Invalid(path.Child("podSecurity").Child("exclude").Index(idx).Child("controlName"), exclude.ControlName, "exclude.images must not be specified for the pod level control"))
				}
			}

			if containsString([]string{"Seccomp", "Capabilities"}, exclude.ControlName) {
				continue
			}

			if containsString(forbiddenControls, exclude.ControlName) {
				errs = append(errs, field.Invalid(path.Child("podSecurity").Child("exclude").Index(idx).Child("controlName"), exclude.ControlName, "Invalid control name defined at the given level"))
			}
		}
	}
	return errs
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
	errs = append(errs, r.ValidateMutationRuleTargetNamespace(path, namespaced, policyNamespace)...)
	errs = append(errs, r.ValidatePSaControlNames(path)...)
	errs = append(errs, r.ValidateGenerate(path, namespaced, policyNamespace, clusterResources)...)
	return errs
}
