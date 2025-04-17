package v2beta1

import (
	kjson "github.com/kyverno/kyverno-json/pkg/apis/policy/v1alpha1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

// AssertionTree defines a kyverno-json assertion tree.
type AssertionTree = kjson.Any

// Validation defines checks to be performed on matching resources.
type Validation struct {
	// FailureAction defines if a validation policy rule violation should block
	// the admission review request (Enforce), or allow (Audit) the admission review request
	// and report an error in a policy report. Optional.
	// Allowed values are Audit or Enforce.
	// +optional
	// +kubebuilder:validation:Enum=Audit;Enforce
	FailureAction *kyvernov1.ValidationFailureAction `json:"failureAction,omitempty"`

	// FailureActionOverrides is a Cluster Policy attribute that specifies FailureAction
	// namespace-wise. It overrides FailureAction for the specified namespaces.
	// +optional
	FailureActionOverrides []kyvernov1.ValidationFailureActionOverride `json:"failureActionOverrides,omitempty"`

	// Message specifies a custom message to be displayed on failure.
	// +optional
	Message string `json:"message,omitempty"`

	// Manifest specifies conditions for manifest verification
	// +optional
	Manifests *kyvernov1.Manifests `json:"manifests,omitempty"`

	// ForEach applies validate rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
	// +optional
	ForEachValidation []kyvernov1.ForEachValidation `json:"foreach,omitempty"`

	// Pattern specifies an overlay-style pattern used to check resources.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	RawPattern *kyverno.Any `json:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	RawAnyPattern *kyverno.Any `json:"anyPattern,omitempty"`

	// Deny defines conditions used to pass or fail a validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty"`

	// PodSecurity applies exemptions for Kubernetes Pod Security admission
	// by specifying exclusions for Pod Security Standards controls.
	// +optional
	PodSecurity *kyvernov1.PodSecurity `json:"podSecurity,omitempty"`

	// CEL allows validation checks using the Common Expression Language (https://kubernetes.io/docs/reference/using-api/cel/).
	// +optional
	CEL *kyvernov1.CEL `json:"cel,omitempty"`

	// Assert defines a kyverno-json assertion tree.
	// +optional
	Assert AssertionTree `json:"assert"`
}

// ConditionOperator is the operation performed on condition key and value.
// +kubebuilder:validation:Enum=Equals;NotEquals;AnyIn;AllIn;AnyNotIn;AllNotIn;GreaterThanOrEquals;GreaterThan;LessThanOrEquals;LessThan;DurationGreaterThanOrEquals;DurationGreaterThan;DurationLessThanOrEquals;DurationLessThan
type ConditionOperator string

// ConditionOperators stores all the valid ConditionOperator types as key-value pairs.
// "Equals" evaluates if the key is equal to the value.
// "NotEquals" evaluates if the key is not equal to the value.
// "AnyIn" evaluates if any of the keys are contained in the set of values.
// "AllIn" evaluates if all the keys are contained in the set of values.
// "AnyNotIn" evaluates if any of the keys are not contained in the set of values.
// "AllNotIn" evaluates if all the keys are not contained in the set of values.
// "GreaterThanOrEquals" evaluates if the key (numeric) is greater than or equal to the value (numeric).
// "GreaterThan" evaluates if the key (numeric) is greater than the value (numeric).
// "LessThanOrEquals" evaluates if the key (numeric) is less than or equal to the value (numeric).
// "LessThan" evaluates if the key (numeric) is less than the value (numeric).
// "DurationGreaterThanOrEquals" evaluates if the key (duration) is greater than or equal to the value (duration)
// "DurationGreaterThan" evaluates if the key (duration) is greater than the value (duration)
// "DurationLessThanOrEquals" evaluates if the key (duration) is less than or equal to the value (duration)
// "DurationLessThan" evaluates if the key (duration) is greater than the value (duration)
var ConditionOperators = map[string]ConditionOperator{
	"Equals":                      ConditionOperator("Equals"),
	"NotEquals":                   ConditionOperator("NotEquals"),
	"AnyIn":                       ConditionOperator("AnyIn"),
	"AllIn":                       ConditionOperator("AllIn"),
	"AnyNotIn":                    ConditionOperator("AnyNotIn"),
	"AllNotIn":                    ConditionOperator("AllNotIn"),
	"GreaterThanOrEquals":         ConditionOperator("GreaterThanOrEquals"),
	"GreaterThan":                 ConditionOperator("GreaterThan"),
	"LessThanOrEquals":            ConditionOperator("LessThanOrEquals"),
	"LessThan":                    ConditionOperator("LessThan"),
	"DurationGreaterThanOrEquals": ConditionOperator("DurationGreaterThanOrEquals"),
	"DurationGreaterThan":         ConditionOperator("DurationGreaterThan"),
	"DurationLessThanOrEquals":    ConditionOperator("DurationLessThanOrEquals"),
	"DurationLessThan":            ConditionOperator("DurationLessThan"),
}

// Deny specifies a list of conditions used to pass or fail a validation rule.
type Deny struct {
	// Multiple conditions can be declared under an `any` or `all` statement.
	// See: https://kyverno.io/docs/writing-policies/validate/#deny-rules
	RawAnyAllConditions *AnyAllConditions `json:"conditions,omitempty"`
}

type Condition struct {
	// Key is the context entry (using JMESPath) for conditional rule evaluation.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	RawKey *kyverno.Any `json:"key,omitempty"`

	// Operator is the conditional operation to perform. Valid operators are:
	// Equals, NotEquals, In, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn, GreaterThanOrEquals,
	// GreaterThan, LessThanOrEquals, LessThan, DurationGreaterThanOrEquals, DurationGreaterThan,
	// DurationLessThanOrEquals, DurationLessThan
	Operator ConditionOperator `json:"operator,omitempty"`

	// Value is the conditional value, or set of values. The values can be fixed set
	// or can be variables declared using JMESPath.
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:pruning:PreserveUnknownFields
	RawValue *kyverno.Any `json:"value,omitempty"`

	// Message is an optional display message
	Message string `json:"message,omitempty"`
}

func (c *Condition) GetKey() any {
	return kyverno.FromAny(c.RawKey)
}

func (c *Condition) SetKey(in any) {
	c.RawKey = kyverno.ToAny(in)
}

func (c *Condition) GetValue() any {
	return kyverno.FromAny(c.RawValue)
}

func (c *Condition) SetValue(in any) {
	c.RawValue = kyverno.ToAny(in)
}

type AnyAllConditions struct {
	// AnyConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, at least one of the conditions need to pass.
	// +optional
	AnyConditions []Condition `json:"any,omitempty"`

	// AllConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, all of the conditions need to pass.
	// +optional
	AllConditions []Condition `json:"all,omitempty"`
}

// ResourceFilters is a slice of ResourceFilter
type ResourceFilters []ResourceFilter

// ResourceFilter allow users to "AND" or "OR" between resources
type ResourceFilter struct {
	// UserInfo contains information about the user performing the operation.
	// +optional
	kyvernov1.UserInfo `json:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	ResourceDescription `json:"resources,omitempty"`
}
