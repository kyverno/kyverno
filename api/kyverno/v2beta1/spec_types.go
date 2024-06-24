package v2beta1

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Spec contains a list of Rule instances and other policy controls.
type Spec struct {
	// Rules is a list of Rule instances. A Policy contains multiple rules and
	// each rule can validate, mutate, or generate resources.
	Rules []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`

	// ApplyRules controls how rules in a policy are applied. Rule are processed in
	// the order of declaration. When set to `One` processing stops after a rule has
	// been applied i.e. the rule matches and results in a pass, fail, or error. When
	// set to `All` all rules in the policy are processed. The default is `All`.
	// +optional
	ApplyRules *kyvernov1.ApplyRulesType `json:"applyRules,omitempty" yaml:"applyRules,omitempty"`

	// Deprecated, use failurePolicy under the webhookConfiguration instead.
	FailurePolicy *kyvernov1.FailurePolicyType `json:"failurePolicy,omitempty" yaml:"failurePolicy,omitempty"`

	// Deprecated, use validationFailureAction under the validate rule instead.
	// +kubebuilder:validation:Enum=audit;enforce;Audit;Enforce
	// +kubebuilder:default=Audit
	ValidationFailureAction kyvernov1.ValidationFailureAction `json:"validationFailureAction,omitempty" yaml:"validationFailureAction,omitempty"`

	// Deprecated, use validationFailureActionOverrides under the validate rule instead.
	ValidationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride `json:"validationFailureActionOverrides,omitempty" yaml:"validationFailureActionOverrides,omitempty"`

	// Admission controls if rules are applied during admission.
	// Optional. Default value is "true".
	// +optional
	// +kubebuilder:default=true
	Admission *bool `json:"admission,omitempty" yaml:"admission,omitempty"`

	// Background controls if rules are applied to existing resources during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	// +optional
	// +kubebuilder:default=true
	Background *bool `json:"background,omitempty" yaml:"background,omitempty"`

	// Deprecated.
	SchemaValidation *bool `json:"schemaValidation,omitempty" yaml:"schemaValidation,omitempty"`

	// Deprecated, use webhookTimeoutSeconds under webhookConfiguration instead.
	WebhookTimeoutSeconds *int32 `json:"webhookTimeoutSeconds,omitempty" yaml:"webhookTimeoutSeconds,omitempty"`

	// Deprecated, use mutateExistingOnPolicyUpdate under the mutate rule instead
	// +optional
	MutateExistingOnPolicyUpdate bool `json:"mutateExistingOnPolicyUpdate,omitempty" yaml:"mutateExistingOnPolicyUpdate,omitempty"`

	// Deprecated, use generateExisting instead
	// +optional
	GenerateExistingOnPolicyUpdate *bool `json:"generateExistingOnPolicyUpdate,omitempty" yaml:"generateExistingOnPolicyUpdate,omitempty"`

	// Deprecated, use generateExisting under the generate rule instead
	GenerateExisting bool `json:"generateExisting,omitempty" yaml:"generateExisting,omitempty"`

	// UseServerSideApply controls whether to use server-side apply for generate rules
	// If is set to "true" create & update for generate rules will use apply instead of create/update.
	// Defaults to "false" if not specified.
	// +optional
	UseServerSideApply bool `json:"useServerSideApply,omitempty" yaml:"useServerSideApply,omitempty"`

	// WebhookConfiguration specifies the custom configuration for Kubernetes admission webhookconfiguration.
	// +optional
	WebhookConfiguration *kyvernov1.WebhookConfiguration `json:"webhookConfiguration,omitempty" yaml:"webhookConfiguration,omitempty"`
}

func (s *Spec) CustomWebhookMatchConditions() bool {
	return s.WebhookConfiguration != nil && len(s.WebhookConfiguration.MatchConditions) != 0
}

func (s *Spec) SetRules(rules []Rule) {
	s.Rules = rules
}

// HasMutateOrValidateOrGenerate checks for rule types
func (s *Spec) HasMutateOrValidateOrGenerate() bool {
	for _, rule := range s.Rules {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate rule types
func (s *Spec) HasMutate() bool {
	for _, rule := range s.Rules {
		if rule.HasMutate() {
			return true
		}
	}

	return false
}

// HasMutate checks for standard admission mutate rule
func (s *Spec) HasMutateStandard() bool {
	for _, rule := range s.Rules {
		if rule.HasMutateStandard() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate existing rule types
func (s *Spec) HasMutateExisting() bool {
	for _, rule := range s.Rules {
		if rule.HasMutateExisting() {
			return true
		}
	}
	return false
}

// HasValidate checks for validate rule types
func (s *Spec) HasValidate() bool {
	for _, rule := range s.Rules {
		if rule.HasValidate() {
			return true
		}
	}

	return false
}

// HasGenerate checks for generate rule types
func (s *Spec) HasGenerate() bool {
	for _, rule := range s.Rules {
		if rule.HasGenerate() {
			return true
		}
	}

	return false
}

// HasVerifyImageChecks checks for image verification rules invoked during resource validation
func (s *Spec) HasVerifyImageChecks() bool {
	for _, rule := range s.Rules {
		if rule.HasVerifyImageChecks() {
			return true
		}
	}

	return false
}

// HasVerifyImages checks for image verification rules invoked during resource mutation
func (s *Spec) HasVerifyImages() bool {
	for _, rule := range s.Rules {
		if rule.HasVerifyImages() {
			return true
		}
	}

	return false
}

// HasVerifyManifests checks for image verification rules invoked during resource mutation
func (s *Spec) HasVerifyManifests() bool {
	for _, rule := range s.Rules {
		if rule.HasVerifyManifests() {
			return true
		}
	}

	return false
}

// AdmissionProcessingEnabled checks if admission is set to true
func (s *Spec) AdmissionProcessingEnabled() bool {
	if s.Admission == nil {
		return true
	}

	return *s.Admission
}

// BackgroundProcessingEnabled checks if background is set to true
func (s *Spec) BackgroundProcessingEnabled() bool {
	if s.Background == nil {
		return true
	}

	return *s.Background
}

// GetValidationFailureAction returns the value of the validationFailureAction
func (s *Spec) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	for _, rule := range s.Rules {
		if rule.HasValidate() {
			validationFailureAction := rule.Validation.ValidationFailureAction
			if validationFailureAction != nil {
				return *validationFailureAction
			}
		}
	}
	return s.ValidationFailureAction
}

// GetValidationFailureActionOverrides returns the value of the validationFailureActionOverrides
func (s *Spec) GetValidationFailureActionOverrides() []kyvernov1.ValidationFailureActionOverride {
	for _, rule := range s.Rules {
		if rule.HasValidate() {
			validationFailureActionOverrides := rule.Validation.ValidationFailureActionOverrides
			if len(validationFailureActionOverrides) != 0 {
				return validationFailureActionOverrides
			}
		}
	}
	return s.ValidationFailureActionOverrides
}

// GetMutateExistingOnPolicyUpdate return MutateExistingOnPolicyUpdate set value
func (s *Spec) GetMutateExistingOnPolicyUpdate() bool {
	for _, rule := range s.Rules {
		if rule.HasMutate() {
			isMutateExisting := rule.Mutation.IsMutateExistingOnPolicyUpdate()
			if isMutateExisting != nil {
				return *isMutateExisting
			}
		}
	}
	return s.MutateExistingOnPolicyUpdate
}

// IsGenerateExisting return GenerateExisting set value
func (s *Spec) IsGenerateExisting() bool {
	for _, rule := range s.Rules {
		if rule.HasGenerate() {
			isGenerateExisting := rule.Generation.IsGenerateExisting()
			if isGenerateExisting != nil {
				return *isGenerateExisting
			}
		}
	}
	if s.GenerateExistingOnPolicyUpdate != nil && *s.GenerateExistingOnPolicyUpdate {
		return true
	}
	return s.GenerateExisting
}

// GetFailurePolicy returns the failure policy to be applied
func (s *Spec) GetFailurePolicy(ctx context.Context) kyvernov1.FailurePolicyType {
	if toggle.FromContext(ctx).ForceFailurePolicyIgnore() {
		return kyvernov1.Ignore
	} else if s.WebhookConfiguration != nil && s.WebhookConfiguration.FailurePolicy != nil {
		return *s.WebhookConfiguration.FailurePolicy
	} else if s.FailurePolicy != nil {
		return *s.FailurePolicy
	}
	return kyvernov1.Fail
}

func (s *Spec) GetWebhookTimeoutSeconds() *int32 {
	if s.WebhookConfiguration != nil && s.WebhookConfiguration.TimeoutSeconds != nil {
		return s.WebhookConfiguration.TimeoutSeconds
	}
	if s.WebhookTimeoutSeconds != nil {
		return s.WebhookTimeoutSeconds
	}
	return nil
}

// GetApplyRules returns the apply rules type
func (s *Spec) GetApplyRules() kyvernov1.ApplyRulesType {
	if s.ApplyRules == nil {
		return kyvernov1.ApplyAll
	}
	return *s.ApplyRules
}

// ValidateRuleNames checks if the rule names are unique across a policy
func (s *Spec) ValidateRuleNames(path *field.Path) (errs field.ErrorList) {
	names := sets.New[string]()
	for i, rule := range s.Rules {
		rulePath := path.Index(i)
		if names.Has(rule.Name) {
			errs = append(errs, field.Invalid(rulePath.Child("name"), rule, fmt.Sprintf(`Duplicate rule name: '%s'`, rule.Name)))
		}
		names.Insert(rule.Name)
	}
	return errs
}

// ValidateRules implements programmatic validation of Rules
func (s *Spec) ValidateRules(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	errs = append(errs, s.ValidateRuleNames(path)...)
	for i, rule := range s.Rules {
		errs = append(errs, rule.Validate(path.Index(i), namespaced, policyNamespace, clusterResources)...)
	}
	return errs
}

func (s *Spec) ValidateDeprecatedFields(path *field.Path) (errs field.ErrorList) {
	if s.WebhookTimeoutSeconds != nil && s.WebhookConfiguration != nil && s.WebhookConfiguration.TimeoutSeconds != nil {
		errs = append(errs, field.Forbidden(path.Child("webhookTimeoutSeconds"), "remove the deprecated field and use spec.webhookConfiguration.timeoutSeconds instead"))
	}

	if s.FailurePolicy != nil && s.WebhookConfiguration != nil && s.WebhookConfiguration.FailurePolicy != nil {
		errs = append(errs, field.Forbidden(path.Child("failurePolicy"), "remove the deprecated field and use spec.webhookConfiguration.failurePolicy instead"))
	}

	for _, rule := range s.Rules {
		if rule.HasGenerate() && rule.Generation.IsGenerateExisting() != nil {
			if s.GenerateExistingOnPolicyUpdate != nil {
				errs = append(errs, field.Forbidden(path.Child("generateExistingOnPolicyUpdate"), "remove the deprecated field and use spec.generate[*].generateExisting instead"))
			}
			if s.GenerateExisting {
				errs = append(errs, field.Forbidden(path.Child("generateExisting"), "remove the deprecated field and use spec.generate[*].generateExisting instead"))
			}
		}

		if rule.HasMutate() && rule.Mutation.IsMutateExistingOnPolicyUpdate() != nil {
			if s.MutateExistingOnPolicyUpdate {
				errs = append(errs, field.Forbidden(path.Child("mutateExistingOnPolicyUpdate"), "remove the deprecated field and use spec.mutate[*].mutateExistingOnPolicyUpdate instead"))
			}
		}
	}
	return errs
}

// Validate implements programmatic validation
func (s *Spec) Validate(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	if err := s.ValidateDeprecatedFields(path); err != nil {
		errs = append(errs, err...)
	}
	if s.WebhookTimeoutSeconds != nil && (*s.WebhookTimeoutSeconds < 1 || *s.WebhookTimeoutSeconds > 30) {
		errs = append(errs, field.Invalid(path.Child("webhookTimeoutSeconds"), s.WebhookTimeoutSeconds, "the timeout value must be between 1 and 30 seconds"))
	}
	if s.WebhookConfiguration != nil && s.WebhookConfiguration.TimeoutSeconds != nil && (*s.WebhookConfiguration.TimeoutSeconds < 1 || *s.WebhookConfiguration.TimeoutSeconds > 30) {
		errs = append(errs, field.Invalid(path.Child("webhookConfiguration.timeoutSeconds"), s.WebhookConfiguration.TimeoutSeconds, "the timeout value must be between 1 and 30 seconds"))
	}
	errs = append(errs, s.ValidateRules(path.Child("rules"), namespaced, policyNamespace, clusterResources)...)
	if namespaced && len(s.ValidationFailureActionOverrides) > 0 {
		errs = append(errs, field.Forbidden(path.Child("validationFailureActionOverrides"), "Use of validationFailureActionOverrides is supported only with ClusterPolicy"))
	}
	return errs
}
