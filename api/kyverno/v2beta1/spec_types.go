package v2beta1

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

	// FailurePolicy defines how unexpected policy errors and webhook response timeout errors are handled.
	// Rules within the same policy share the same failure behavior.
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	FailurePolicy *kyvernov1.FailurePolicyType `json:"failurePolicy,omitempty" yaml:"failurePolicy,omitempty"`

	// ValidationFailureAction defines if a validation policy rule violation should block
	// the admission review request (enforce), or allow (audit) the admission review request
	// and report an error in a policy report. Optional.
	// Allowed values are audit or enforce. The default value is "audit".
	// +optional
	// +kubebuilder:validation:Enum=audit;enforce
	ValidationFailureAction kyvernov1.ValidationFailureAction `json:"validationFailureAction,omitempty" yaml:"validationFailureAction,omitempty"`

	// ValidationFailureActionOverrides is a Cluster Policy attribute that specifies ValidationFailureAction
	// namespace-wise. It overrides ValidationFailureAction for the specified namespaces.
	// +optional
	ValidationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride `json:"validationFailureActionOverrides,omitempty" yaml:"validationFailureActionOverrides,omitempty"`

	// Background controls if rules are applied to existing resources during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	// +optional
	// +kubebuilder:default=true
	Background *bool `json:"background,omitempty" yaml:"background,omitempty"`

	// SchemaValidation skips policy validation checks.
	// Optional. The default value is set to "true", it must be set to "false" to disable the validation checks.
	// +optional
	SchemaValidation *bool `json:"schemaValidation,omitempty" yaml:"schemaValidation,omitempty"`

	// WebhookTimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	WebhookTimeoutSeconds *int32 `json:"webhookTimeoutSeconds,omitempty" yaml:"webhookTimeoutSeconds,omitempty"`

	// MutateExistingOnPolicyUpdate controls if a mutateExisting policy is applied on policy events.
	// Default value is "false".
	// +optional
	MutateExistingOnPolicyUpdate bool `json:"mutateExistingOnPolicyUpdate,omitempty" yaml:"mutateExistingOnPolicyUpdate,omitempty"`

	// GenerateExistingOnPolicyUpdate controls whether to trigger generate rule in existing resources
	// If is set to "true" generate rule will be triggered and applied to existing matched resources.
	// Defaults to "false" if not specified.
	// +optional
	GenerateExistingOnPolicyUpdate bool `json:"generateExistingOnPolicyUpdate,omitempty" yaml:"generateExistingOnPolicyUpdate,omitempty"`
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

// HasImagesValidationChecks checks for image verification rules invoked during resource validation
func (s *Spec) HasImagesValidationChecks() bool {
	for _, rule := range s.Rules {
		if rule.HasImagesValidationChecks() {
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

// HasYAMLSignatureVerify checks for image verification rules invoked during resource mutation
func (s *Spec) HasYAMLSignatureVerify() bool {
	for _, rule := range s.Rules {
		if rule.HasYAMLSignatureVerify() {
			return true
		}
	}

	return false
}

// BackgroundProcessingEnabled checks if background is set to true
func (s *Spec) BackgroundProcessingEnabled() bool {
	if s.Background == nil {
		return true
	}

	return *s.Background
}

// IsMutateExisting checks if the mutate policy applies to existing resources
func (s *Spec) IsMutateExisting() bool {
	for _, rule := range s.Rules {
		if rule.IsMutateExisting() {
			return true
		}
	}
	return false
}

// GetMutateExistingOnPolicyUpdate return MutateExistingOnPolicyUpdate set value
func (s *Spec) GetMutateExistingOnPolicyUpdate() bool {
	return s.MutateExistingOnPolicyUpdate
}

// IsGenerateExistingOnPolicyUpdate return GenerateExistingOnPolicyUpdate set value
func (s *Spec) IsGenerateExistingOnPolicyUpdate() bool {
	return s.GenerateExistingOnPolicyUpdate
}

// GetFailurePolicy returns the failure policy to be applied
func (s *Spec) GetFailurePolicy() kyvernov1.FailurePolicyType {
	if s.FailurePolicy == nil {
		return kyvernov1.Fail
	}
	return *s.FailurePolicy
}

// GetValidationFailureAction returns the validation failure action to be applied
func (s *Spec) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	if s.ValidationFailureAction == "" {
		return kyvernov1.Audit
	}

	return s.ValidationFailureAction
}

// GetFailurePolicy returns the failure policy to be applied
func (s *Spec) GetApplyRules() kyvernov1.ApplyRulesType {
	if s.ApplyRules == nil {
		return kyvernov1.ApplyAll
	}
	return *s.ApplyRules
}

// ValidateRuleNames checks if the rule names are unique across a policy
func (s *Spec) ValidateRuleNames(path *field.Path) (errs field.ErrorList) {
	names := sets.NewString()
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
func (s *Spec) ValidateRules(path *field.Path, namespaced bool, clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, s.ValidateRuleNames(path)...)
	for i, rule := range s.Rules {
		errs = append(errs, rule.Validate(path.Index(i), namespaced, clusterResources)...)
	}
	return errs
}

// Validate implements programmatic validation
func (s *Spec) Validate(path *field.Path, namespaced bool, clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, s.ValidateRules(path.Child("rules"), namespaced, clusterResources)...)
	if namespaced && len(s.ValidationFailureActionOverrides) > 0 {
		errs = append(errs, field.Forbidden(path.Child("validationFailureActionOverrides"), "Use of validationFailureActionOverrides is supported only with ClusterPolicy"))
	}
	return errs
}
