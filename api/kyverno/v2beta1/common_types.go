package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Validation defines checks to be performed on matching resources.
type Validation struct {
	// Message specifies a custom message to be displayed on failure.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Manifest specifies conditions for manifest verification
	// +optional
	Manifests *kyvernov1.Manifests `json:"manifests,omitempty" yaml:"manifests,omitempty"`

	// ForEach applies validate rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
	// +optional
	ForEachValidation []kyvernov1.ForEachValidation `json:"foreach,omitempty" yaml:"foreach,omitempty"`

	// Pattern specifies an overlay-style pattern used to check resources.
	// +optional
	RawPattern *apiextv1.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +optional
	RawAnyPattern *apiextv1.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Deny defines conditions used to pass or fail a validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`

	// PodSecurity applies exemptions for Kubernetes Pod Security admission
	// by specifying exclusions for Pod Security Standards controls.
	// +optional
	PodSecurity *kyvernov1.PodSecurity `json:"podSecurity,omitempty" yaml:"podSecurity,omitempty"`

	// CEL allows validation checks using the Common Expression Language (https://kubernetes.io/docs/reference/using-api/cel/).
	// +optional
	CEL *kyvernov1.CEL `json:"cel,omitempty" yaml:"cel,omitempty"`
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
	RawAnyAllConditions *AnyAllConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

type Condition struct {
	// Key is the context entry (using JMESPath) for conditional rule evaluation.
	RawKey *apiextv1.JSON `json:"key,omitempty" yaml:"key,omitempty"`

	// Operator is the conditional operation to perform. Valid operators are:
	// Equals, NotEquals, In, AnyIn, AllIn, NotIn, AnyNotIn, AllNotIn, GreaterThanOrEquals,
	// GreaterThan, LessThanOrEquals, LessThan, DurationGreaterThanOrEquals, DurationGreaterThan,
	// DurationLessThanOrEquals, DurationLessThan
	Operator ConditionOperator `json:"operator,omitempty" yaml:"operator,omitempty"`

	// Value is the conditional value, or set of values. The values can be fixed set
	// or can be variables declared using JMESPath.
	// +optional
	RawValue *apiextv1.JSON `json:"value,omitempty" yaml:"value,omitempty"`

	// Message is an optional display message
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

func (c *Condition) GetKey() apiextensions.JSON {
	return kyvernov1.FromJSON(c.RawKey)
}

func (c *Condition) SetKey(in apiextensions.JSON) {
	c.RawKey = kyvernov1.ToJSON(in)
}

func (c *Condition) GetValue() apiextensions.JSON {
	return kyvernov1.FromJSON(c.RawValue)
}

func (c *Condition) SetValue(in apiextensions.JSON) {
	c.RawValue = kyvernov1.ToJSON(in)
}

type AnyAllConditions struct {
	// AnyConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, at least one of the conditions need to pass.
	// +optional
	AnyConditions []Condition `json:"any,omitempty" yaml:"any,omitempty"`

	// AllConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, all of the conditions need to pass.
	// +optional
	AllConditions []Condition `json:"all,omitempty" yaml:"all,omitempty"`
}

// ResourceFilters is a slice of ResourceFilter
type ResourceFilters []ResourceFilter

// ResourceFilter allow users to "AND" or "OR" between resources
type ResourceFilter struct {
	// UserInfo contains information about the user performing the operation.
	// +optional
	kyvernov1.UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}
