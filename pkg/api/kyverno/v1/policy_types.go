package v1

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyList is a list of Policy instances.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PolicyList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []Policy `json:"items" yaml:"items"`
}

// Policy declares validation, mutation, and generation behaviors for matching resources.
// See: https://kyverno.io/docs/writing-policies/ for more information.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Background",type="string",JSONPath=".spec.background"
// +kubebuilder:printcolumn:name="Validation Failure Action",type="string",JSONPath=".spec.validationFailureAction"
// +kubebuilder:resource:shortName=pol
type Policy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec defines policy behaviors and contains one or rules.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains policy runtime information.
	// +optional
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// Spec contains a list of Rule instances and other policy controls.
type Spec struct {

	// Rules is a list of Rule instances. A Policy contains multiple rules and
	// each rule can validate, mutate, or generate resources.
	Rules []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`

	// ValidationFailureAction controls if a validation policy rule failure should disallow
	// the admission review request (enforce), or allow (audit) the admission review request
	// and report an error in a policy report. Optional. The default value is "audit".
	// +optional
	ValidationFailureAction string `json:"validationFailureAction,omitempty" yaml:"validationFailureAction,omitempty"`

	// Background controls if rules are applied to existing resources during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	// +optional
	Background *bool `json:"background,omitempty" yaml:"background,omitempty"`
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
	MatchResources MatchResources `json:"match,omitempty" yaml:"match,omitempty"`

	// ExcludeResources defines when this policy rule should not be applied. The exclude
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the name or role.
	// +optional
	ExcludeResources ExcludeResources `json:"exclude,omitempty" yaml:"exclude,omitempty"`

	// AnyAllConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// This too can be made to happen in a logical-manner where in some situation all the conditions need to pass
	// and in some other situation, atleast one condition is enough to pass.
	// For the sake of backwards compatibility, it can be populated with []kyverno.Condition.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyAllConditions apiextensions.JSON `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// Mutation is used to modify matching resources.
	// +optional
	Mutation Mutation `json:"mutate,omitempty" yaml:"mutate,omitempty"`

	// Validation is used to validate matching resources.
	// +optional
	Validation Validation `json:"validate,omitempty" yaml:"validate,omitempty"`

	// Generation is used to create new resources.
	// +optional
	Generation Generation `json:"generate,omitempty" yaml:"generate,omitempty"`
}

// AnyAllCondition consists of conditions wrapped denoting a logical criteria to be fulfilled.
// AnyConditions get fulfilled when at least one of its sub-conditions passes.
// AllConditions get fulfilled only when all of its sub-conditions pass.
type AnyAllConditions struct {
	// AnyConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, atleast one of the conditions need to pass
	// +optional
	AnyConditions []Condition `json:"any,omitempty" yaml:"any,omitempty"`

	// AllConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, all of the conditions need to pass
	// +optional
	AllConditions []Condition `json:"all,omitempty" yaml:"all,omitempty"`
}

// ContextEntry adds variables and data sources to a rule Context. Either a
// ConfigMap reference or a APILookup must be provided.
type ContextEntry struct {

	// Name is the variable name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// ConfigMap is the ConfigMap reference.
	ConfigMap *ConfigMapReference `json:"configMap,omitempty" yaml:"configMap,omitempty"`

	// APICall defines an HTTP request to the Kubernetes API server. The JSON
	// data retrieved is stored in the context.
	APICall *APICall `json:"apiCall,omitempty" yaml:"apiCall,omitempty"`
}

// ConfigMapReference refers to a ConfigMap
type ConfigMapReference struct {

	// Name is the ConfigMap name.
	Name string `json:"name" yaml:"name"`

	// Namespace is the ConfigMap namespace.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// APICall defines an HTTP request to the Kubernetes API server. The JSON
// data retrieved is stored in the context. An APICall contains a URLPath
// used to perform the HTTP GET request and an optional JMESPath used to
// transform the retrieved JSON data.
type APICall struct {

	// URLPath is the URL path to be used in the HTTP GET request to the
	// Kubernetes API server (e.g. "/api/v1/namespaces" or  "/apis/apps/v1/deployments").
	// The format required is the same format used by the `kubectl get --raw` command.
	URLPath string `json:"urlPath" yaml:"urlPath"`

	// JMESPath is an optional JSON Match Expression that can be used to
	// transform the JSON response returned from the API server. For example
	// a JMESPath of "items | length(@)" applied to the API server response
	// to the URLPath "/apis/apps/v1/deployments" will return the total count
	// of deployments across all namespaces.
	// +optional
	JMESPath string `json:"jmesPath,omitempty" yaml:"jmesPath,omitempty"`
}

// Condition defines variable-based conditional criteria for rule execution.
type Condition struct {
	// Key is the context entry (using JMESPath) for conditional rule evaluation.
	// +kubebuilder:validation:XPreserveUnknownFields
	Key apiextensions.JSON `json:"key,omitempty" yaml:"key,omitempty"`

	// Operator is the operation to perform. Valid operators
	// are Equals, NotEquals, In and NotIn.
	Operator ConditionOperator `json:"operator,omitempty" yaml:"operator,omitempty"`

	// Value is the conditional value, or set of values. The values can be fixed set
	// or can be variables declared using using JMESPath.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Value apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

// ConditionOperator is the operation performed on condition key and value.
// +kubebuilder:validation:Enum=Equals;NotEquals;In;NotIn
type ConditionOperator string

const (
	// Equal evaluates if the key is equal to the value.
	// Deprecated. Use Equals instead.
	Equal ConditionOperator = "Equal"
	// Equals evaluates if the key is equal to the value.
	Equals ConditionOperator = "Equals"
	// NotEqual evaluates if the key is not equal to the value.
	// Deprecated. Use NotEquals instead.
	NotEqual ConditionOperator = "NotEqual"
	// NotEquals evaluates if the key is not equal to the value.
	NotEquals ConditionOperator = "NotEquals"
	// In evaluates if the key is contained in the set of values.
	In ConditionOperator = "In"
	// NotIn evaluates if the key is not contained in the set of values.
	NotIn ConditionOperator = "NotIn"
	// GreaterThanOrEquals evaluates if the key (numeric) is greater than or equal to the value (numeric).
	GreaterThanOrEquals ConditionOperator = "GreaterThanOrEquals"
	// GreaterThan evaluates if the key (numeric) is greater than the value (numeric).
	GreaterThan ConditionOperator = "GreaterThan"
	// LessThanOrEquals evaluates if the key (numeric) is less than or equal to the value (numeric).
	LessThanOrEquals ConditionOperator = "LessThanOrEquals"
	// LessThan evaluates if the key (numeric) is less than the value (numeric).
	LessThan ConditionOperator = "LessThan"
)

// MatchResources is used to specify resource and admission review request data for
// which a policy rule is applicable.
type MatchResources struct {
	// UserInfo contains information about the user performing the operation.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	// Requires at least one tag to be specified when under MatchResources.
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ExcludeResources specifies resource and admission review request data for
// which a policy rule is not applicable.
type ExcludeResources struct {
	// UserInfo contains information about the user performing the operation.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// UserInfo contains information about the user performing the operation.
type UserInfo struct {
	// Roles is the list of namespaced role names for the user.
	// +optional
	Roles []string `json:"roles,omitempty" yaml:"roles,omitempty"`

	// ClusterRoles is the list of cluster-wide role names for the user.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty" yaml:"clusterRoles,omitempty"`

	// Subjects is the list of subject names like users, user groups, and service accounts.
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty" yaml:"subjects,omitempty"`
}

// ResourceDescription contains criteria used to match resources.
type ResourceDescription struct {
	// Kinds is a list of resource kinds.
	// +optional
	Kinds []string `json:"kinds,omitempty" yaml:"kinds,omitempty"`

	// Name is the name of the resource. The name supports wildcard characters
	// "*" (matches zero or many characters) and "?" (at least one character).
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Namespaces is a list of namespaces names. Each name supports wildcard characters
	// "*" (matches zero or many characters) and "?" (at least one character).
	// +optional
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`

	// Annotations is a  map of annotations (key-value pairs of type string). Annotation keys
	// and values support the wildcard characters "*" (matches zero or many characters) and
	// "?" (matches at least one character).
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Selector is a label selector. Label keys and values in `matchLabels` support the wildcard
	// characters `*` (matches zero or many characters) and `?` (matches one character).
	// Wildcards allows writing label selectors like ["storage.k8s.io/*": "*"]. Note that
	// using ["*" : "*"] matches any key and value but does not match an empty label set.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`

	// NamespaceSelector is a label selector for the resource namespace. Label keys and values
	// in `matchLabels` support the wildcard characters `*` (matches zero or many characters)
	// and `?` (matches one character).Wildcards allows writing label selectors like
	// ["storage.k8s.io/*": "*"]. Note that using ["*" : "*"] matches any key and value but
	// does not match an empty label set.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`
}

// Mutation defines how resource are modified.
type Mutation struct {
	// Overlay specifies an overlay pattern to modify resources.
	// DEPRECATED. Use PatchStrategicMerge instead. Scheduled for
	// removal in release 1.5+.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Overlay apiextensions.JSON `json:"overlay,omitempty"`

	// Patches specifies a RFC 6902 JSON Patch to modify resources.
	// DEPRECATED. Use PatchesJSON6902 instead. Scheduled for
	// removal in release 1.5+.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +nullable
	// +optional
	Patches []Patch `json:"patches,omitempty" yaml:"patches,omitempty"`

	// PatchStrategicMerge is a strategic merge patch used to modify resources.
	// See https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/
	// and https://kubectl.docs.kubernetes.io/references/kustomize/patchesstrategicmerge/.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	PatchStrategicMerge apiextensions.JSON `json:"patchStrategicMerge,omitempty" yaml:"patchStrategicMerge,omitempty"`

	// PatchesJSON6902 is a list of RFC 6902 JSON Patch declarations used to modify resources.
	// See https://tools.ietf.org/html/rfc6902 and https://kubectl.docs.kubernetes.io/references/kustomize/patchesjson6902/.
	// +optional
	PatchesJSON6902 string `json:"patchesJson6902,omitempty" yaml:"patchesJson6902,omitempty"`
}

// +k8s:deepcopy-gen=false

// Patch is a RFC 6902 JSON Patch.
// See: https://tools.ietf.org/html/rfc6902
type Patch struct {

	// Path specifies path of the resource.
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Operation specifies operations supported by JSON Patch.
	// i.e:- add, replace and delete.
	Operation string `json:"op,omitempty" yaml:"op,omitempty"`

	// Value specifies the value to be applied.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Value apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

// Validation defines checks to be performed on matching resources.
type Validation struct {

	// Message specifies a custom message to be displayed on failure.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Pattern specifies an overlay-style pattern used to check resources.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Pattern apiextensions.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyPattern apiextensions.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Deny defines conditions to fail the validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`
}

// Deny specifies a list of conditions. The validation rule fails, if any Condition
// evaluates to "false".
type Deny struct {
	// specifies the set of conditions to deny in a logical manner
	// For the sake of backwards compatibility, it can be populated with []kyverno.Condition.
	// +kubebuilder:validation:XPreserveUnknownFields
	AnyAllConditions apiextensions.JSON `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Generation defines how new resources should be created and managed.
type Generation struct {

	// ResourceSpec contains information to select the resource.
	ResourceSpec `json:",omitempty" yaml:",omitempty"`

	// Synchronize controls if generated resources should be kept in-sync with their source resource.
	// If Synchronize is set to "true" changes to generated resources will be overwritten with resource
	// data from Data or the resource specified in the Clone declaration.
	// Optional. Defaults to "false" if not specified.
	// +optional
	Synchronize bool `json:"synchronize,omitempty" yaml:"synchronize,omitempty"`

	// Data provides the resource declaration used to populate each generated resource.
	// At most one of Data or Clone must be specified. If neither are provided, the generated
	// resource will be created with default data only.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Data apiextensions.JSON `json:"data,omitempty" yaml:"data,omitempty"`

	// Clone specifies the source resource used to populate each generated resource.
	// At most one of Data or Clone can be specified. If neither are provided, the generated
	// resource will be created with default data only.
	// +optional
	Clone CloneFrom `json:"clone,omitempty" yaml:"clone,omitempty"`
}

// CloneFrom provides the location of the source resource used to generate target resources.
// The resource kind is derived from the match criteria.
type CloneFrom struct {

	// Namespace specifies source resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Name specifies name of the resource.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// PolicyStatus mostly contains runtime information related to policy execution.
type PolicyStatus struct {
	// AvgExecutionTime is the average time taken to process the policy rules on a resource.
	// +optional
	AvgExecutionTime string `json:"averageExecutionTime,omitempty" yaml:"averageExecutionTime,omitempty"`

	// ViolationCount is the total count of policy failure results for this policy.
	// +optional
	ViolationCount int `json:"violationCount,omitempty" yaml:"violationCount,omitempty"`

	// RulesFailedCount is the total count of policy execution errors for this policy.
	// +optional
	RulesFailedCount int `json:"rulesFailedCount,omitempty" yaml:"rulesFailedCount,omitempty"`

	// RulesAppliedCount is the total number of times this policy was applied.
	// +optional
	RulesAppliedCount int `json:"rulesAppliedCount,omitempty" yaml:"rulesAppliedCount,omitempty"`

	// ResourcesBlockedCount is the total count of admission review requests that were blocked by this policy.
	// +optional
	ResourcesBlockedCount int `json:"resourcesBlockedCount,omitempty" yaml:"resourcesBlockedCount,omitempty"`

	// ResourcesMutatedCount is the total count of resources that were mutated by this policy.
	// +optional
	ResourcesMutatedCount int `json:"resourcesMutatedCount,omitempty" yaml:"resourcesMutatedCount,omitempty"`

	// ResourcesGeneratedCount is the total count of resources that were generated by this policy.
	// +optional
	ResourcesGeneratedCount int `json:"resourcesGeneratedCount,omitempty" yaml:"resourcesGeneratedCount,omitempty"`

	// Rules provides per rule statistics
	// +optional
	Rules []RuleStats `json:"ruleStatus,omitempty" yaml:"ruleStatus,omitempty"`
}

// RuleStats provides statistics for an individual rule within a policy.
type RuleStats struct {
	// Name is the rule name.
	Name string `json:"ruleName" yaml:"ruleName"`

	// ExecutionTime is the average time taken to execute this rule.
	// +optional
	ExecutionTime string `json:"averageExecutionTime,omitempty" yaml:"averageExecutionTime,omitempty"`

	// ViolationCount is the total count of policy failure results for this rule.
	// +optional
	ViolationCount int `json:"violationCount,omitempty" yaml:"violationCount,omitempty"`

	// FailedCount is the total count of policy error results for this rule.
	// +optional
	FailedCount int `json:"failedCount,omitempty" yaml:"failedCount,omitempty"`

	// AppliedCount is the total number of times this rule was applied.
	// +optional
	AppliedCount int `json:"appliedCount,omitempty" yaml:"appliedCount,omitempty"`

	// ResourcesBlockedCount is the total count of admission review requests that were blocked by this rule.
	// +optional
	ResourcesBlockedCount int `json:"resourcesBlockedCount,omitempty" yaml:"resourcesBlockedCount,omitempty"`

	// ResourcesMutatedCount is the total count of resources that were mutated by this rule.
	// +optional
	ResourcesMutatedCount int `json:"resourcesMutatedCount,omitempty" yaml:"resourcesMutatedCount,omitempty"`

	// ResourcesGeneratedCount is the total count of resources that were generated by this rule.
	// +optional
	ResourcesGeneratedCount int `json:"resourcesGeneratedCount,omitempty" yaml:"resourcesGeneratedCount,omitempty"`
}

// ResourceSpec contains information to identify a resource.
type ResourceSpec struct {
	// APIVersion specifies resource apiVersion.
	// +optional
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	// Kind specifies resource kind.
	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`
	// Namespace specifies resource namespace.
	// +optional
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Name specifies the resource name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
