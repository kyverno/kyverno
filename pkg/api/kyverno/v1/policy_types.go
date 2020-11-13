package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyList ...
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []Policy `json:"items" yaml:"items"`
}

// Policy contains rules to be applied to created resources.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Background",type="string",JSONPath=".spec.background"
// +kubebuilder:printcolumn:name="Validatoin Failure Action",type="string",JSONPath=".spec.validationFailureAction"
// +kubebuilder:resource:shortName=pol
type Policy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec is the information to identify the policy.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains statistics related to policy.
	// +optional
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// Spec describes policy behavior by its rules.
type Spec struct {
	// Rules contains the list of rules to be applied to resources.
	Rules []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`
	// ValidationFailureAction controls if a policy failure should not disallow
	// an admission review request (enforce), or allow (audit) and report an error.
	// Default value is "audit".
	// +kubebuilder:default=audit
	// +optional
	ValidationFailureAction string `json:"validationFailureAction,omitempty" yaml:"validationFailureAction,omitempty"`

	// Background controls if rules are applied to existing resources during a background scan.
	// Default value is "true".
	// +kubebuilder:default=true
	// +optional
	Background *bool `json:"background,omitempty" yaml:"background,omitempty"`
}

// Rule contains a mutation, validation, or generation action
// for the single resource description.
type Rule struct {
	// A unique label for the rule.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Defines variables that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// Selects resources for which the policy rule should be applied.
	// If it's defined, "kinds" inside MatchResources block is required.
	// +optional
	MatchResources MatchResources `json:"match,omitempty" yaml:"match,omitempty"`

	// Selects resources for which the policy rule should not be applied.
	// +optional
	ExcludeResources ExcludeResources `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	// Allows condition-based control of the policy rule execution.
	// +optional
	Conditions []Condition `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// Modifies matching resources.
	// +optional
	Mutation Mutation `json:"mutate,omitempty" yaml:"mutate,omitempty"`

	// Checks matching resources.
	// +optional
	Validation Validation `json:"validate,omitempty" yaml:"validate,omitempty"`

	// Generates new resources.
	// +optional
	Generation Generation `json:"generate,omitempty" yaml:"generate,omitempty"`
}

type ContextEntry struct {
	Name      string              `json:"name,omitempty" yaml:"name,omitempty"`
	ConfigMap *ConfigMapReference `json:"configMap,omitempty" yaml:"configMap,omitempty"`
}

type ConfigMapReference struct {
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// Condition defines the evaluation condition.
type Condition struct {
	// Key contains key to compare.
	// +kubebuilder:validation:XPreserveUnknownFields
	Key apiextensions.JSON `json:"key,omitempty" yaml:"key,omitempty"`

	// Operator to compare against value.
	Operator ConditionOperator `json:"operator,omitempty" yaml:"operator,omitempty"`

	// Value to be compared.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Value apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

// ConditionOperator defines the type for condition operator.
type ConditionOperator string

const (
	Equal     ConditionOperator = "Equal"
	Equals    ConditionOperator = "Equals"
	NotEqual  ConditionOperator = "NotEqual"
	NotEquals ConditionOperator = "NotEquals"
	In        ConditionOperator = "In"
	NotIn     ConditionOperator = "NotIn"
)

// MatchResources contains resource description of the resources that the rule is to apply on.
type MatchResources struct {
	// Specifies user information.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// Specifies resources to which rule is applied.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ExcludeResources container resource description of the resources that are to be excluded from the applying the policy rule.
type ExcludeResources struct {
	// Specifies user information.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// Specifies resources to which rule is excluded.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// UserInfo filter based on users.
type UserInfo struct {
	// Specifies list of namespaced role names.
	// +optional
	Roles []string `json:"roles,omitempty" yaml:"roles,omitempty"`

	// Specifies list of cluster wide role names.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty" yaml:"clusterRoles,omitempty"`

	// Specifies list of subject names like users, user groups, and service accounts.
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty" yaml:"subjects,omitempty"`
}

// ResourceDescription describes the resource to which the PolicyRule will be applied.
type ResourceDescription struct {
	// Specifies list of resource kind.
	// +optional
	Kinds []string `json:"kinds,omitempty" yaml:"kinds,omitempty"`

	// Specifies name of the resource.
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Specifies list of namespaces.
	// +optional
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`

	// Specifies map of annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Specifies the set of selectors.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
}

// Mutation describes the way how Mutating Webhook will react on resource creation.
type Mutation struct {
	// Specifies overlay patterns.
	// Overlay is preserved for backwards compatibility and will be removed in Kyverno 1.5+.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Overlay apiextensions.JSON `json:"overlay,omitempty"`

	// Specifies JSON Patch.
	// Patches is preserved for backwards compatibility and will be removed in Kyverno 1.5+.
	// +optional
	Patches []Patch `json:"patches,omitempty" yaml:"patches,omitempty"`

	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	PatchStrategicMerge apiextensions.JSON `json:"patchStrategicMerge,omitempty" yaml:"patchStrategicMerge,omitempty"`

	// +optional
	PatchesJSON6902 string `json:"patchesJson6902,omitempty" yaml:"patchesJson6902,omitempty"`
}

// +k8s:deepcopy-gen=false

// Patch declares patch operation for created object according to RFC 6902.
type Patch struct {
	// Specifies path of the resource.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
	// Specifies operations supported by JSON Patch.
	// i.e:- add, replace and delete.
	Operation string `json:"op,omitempty" yaml:"op,omitempty"`

	// Specifies the value to be applied.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Value apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

// Validation describes the way how Validating Webhook will check the resource on creation.
type Validation struct {
	// Specifies message to be displayed on validation policy violation.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Specifies validation pattern.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Pattern apiextensions.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// Specifies list of validation patterns.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyPattern apiextensions.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Specifies conditions to deny validation.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`
}

type Deny struct {
	// Specifies set of condition to deny validation.
	Conditions []Condition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Generation describes which resources will be created when other resource is created.
type Generation struct {
	ResourceSpec `json:",omitempty" yaml:",omitempty"`

	// To keep resources synchronized with source resource.
	// +optional
	Synchronize bool `json:"synchronize,omitempty" yaml:"synchronize,omitempty"`

	// Data specifies the resource manifest to be generated.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Data apiextensions.JSON `json:"data,omitempty" yaml:"data,omitempty"`

	// To clone resource from other resource.
	// +optional
	Clone CloneFrom `json:"clone,omitempty" yaml:"clone,omitempty"`
}

// CloneFrom - location of the resource,
// which will be used as source when applying 'generate'.
type CloneFrom struct {
	// Specifies resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Specifies name of the resource.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// PolicyStatus mostly contains statistics related to policy.
type PolicyStatus struct {
	// Average time required to process the policy rules on a resource.
	// +optional
	AvgExecutionTime string `json:"averageExecutionTime,omitempty" yaml:"averageExecutionTime,omitempty"`

	// Number of violations created by this policy.
	// +optional
	ViolationCount int `json:"violationCount,omitempty" yaml:"violationCount,omitempty"`

	// Count of rules that failed.
	// +optional
	RulesFailedCount int `json:"rulesFailedCount,omitempty" yaml:"rulesFailedCount,omitempty"`

	// Count of rules that were applied.
	// +optional
	RulesAppliedCount int `json:"rulesAppliedCount,omitempty" yaml:"rulesAppliedCount,omitempty"`

	// Count of resources that were blocked for failing a validate, across all rules.
	// +optional
	ResourcesBlockedCount int `json:"resourcesBlockedCount,omitempty" yaml:"resourcesBlockedCount,omitempty"`

	// Count of resources that were successfully mutated, across all rules.
	// +optional
	ResourcesMutatedCount int `json:"resourcesMutatedCount,omitempty" yaml:"resourcesMutatedCount,omitempty"`

	// Count of resources that were successfully generated, across all rules.
	// +optional
	ResourcesGeneratedCount int `json:"resourcesGeneratedCount,omitempty" yaml:"resourcesGeneratedCount,omitempty"`

	// +optional
	Rules []RuleStats `json:"ruleStatus,omitempty" yaml:"ruleStatus,omitempty"`
}

// RuleStats provides status per rule.
type RuleStats struct {
	// Rule name.
	Name string `json:"ruleName" yaml:"ruleName"`

	// Average time require to process the rule.
	// +optional
	ExecutionTime string `json:"averageExecutionTime,omitempty" yaml:"averageExecutionTime,omitempty"`

	// Number of violations created by this rule.
	// +optional
	ViolationCount int `json:"violationCount,omitempty" yaml:"violationCount,omitempty"`

	// Count of rules that failed.
	// +optional
	FailedCount int `json:"failedCount,omitempty" yaml:"failedCount,omitempty"`

	// Count of rules that were applied.
	// +optional
	AppliedCount int `json:"appliedCount,omitempty" yaml:"appliedCount,omitempty"`

	// Count of resources for whom update/create api requests were blocked as the resource did not satisfy the policy rules.
	// +optional
	ResourcesBlockedCount int `json:"resourcesBlockedCount,omitempty" yaml:"resourcesBlockedCount,omitempty"`

	// Count of resources that were successfully mutated.
	// +optional
	ResourcesMutatedCount int `json:"resourcesMutatedCount,omitempty" yaml:"resourcesMutatedCount,omitempty"`

	// Count of resources that were successfully generated.
	// +optional
	ResourcesGeneratedCount int `json:"resourcesGeneratedCount,omitempty" yaml:"resourcesGeneratedCount,omitempty"`
}

// ResourceSpec information to identify the resource.
type ResourceSpec struct {
	// Specifies resource apiVersion.
	// +optional
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	// Specifies resource kind.
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	// Specifies resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Specifies resource name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// ViolatedRule stores the information regarding the rule.
type ViolatedRule struct {
	// Specifies violated rule name.
	Name string `json:"name" yaml:"name"`

	// Specifies violated rule type.
	Type string `json:"type" yaml:"type"`

	// Specifies violation message.
	// +optional
	Message string `json:"message" yaml:"message"`

	// +optional
	Check string `json:"check" yaml:"check"`
}
