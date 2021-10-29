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
// +kubebuilder:printcolumn:name="Action",type="string",JSONPath=".spec.validationFailureAction"
// +kubebuilder:printcolumn:name="Failure Policy",type="string",JSONPath=".spec.failurePolicy",priority=1
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.ready`
// +kubebuilder:resource:shortName=pol
type Policy struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec defines policy behaviors and contains one or more rules.
	Spec Spec `json:"spec" yaml:"spec"`

	// Status contains policy runtime information.
	// +optional
	// Deprecated. Policy metrics are available via the metrics endpoint
	Status PolicyStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// Spec contains a list of Rule instances and other policy controls.
type Spec struct {

	// Rules is a list of Rule instances. A Policy contains multiple rules and
	// each rule can validate, mutate, or generate resources.
	Rules []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`

	// FailurePolicy defines how unrecognized errors from the admission endpoint are handled.
	// Rules within the same policy share the same failure behavior.
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	FailurePolicy *FailurePolicyType `json:"failurePolicy,omitempty" yaml:"failurePolicy,omitempty"`

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

	// SchemaValidation skips policy validation checks.
	// Optional. The default value is set to "true", it must be set to "false" to disable the validation checks.
	// +optional
	SchemaValidation *bool `json:"schemaValidation,omitempty" yaml:"schemaValidation,omitempty"`

	// WebhookTimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	WebhookTimeoutSeconds *int32 `json:"webhookTimeoutSeconds,omitempty" yaml:"webhookTimeoutSeconds,omitempty"`
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

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements. A direct list
	// of conditions (without `any` or `all` statements is supported for backwards compatibility but
	// will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
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

	// VerifyImages is used to verify image signatures and mutate them to add a digest
	// +optional
	VerifyImages []*ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`
}

// FailurePolicyType specifies a failure policy that defines how unrecognized errors from the admission endpoint are handled.
// +kubebuilder:validation:Enum=Ignore;Fail
type FailurePolicyType string

const (
	// Ignore means that an error calling the webhook is ignored.
	Ignore FailurePolicyType = "Ignore"
	// Fail means that an error calling the webhook causes the admission to fail.
	Fail FailurePolicyType = "Fail"
)

// AnyAllConditions consists of conditions wrapped denoting a logical criteria to be fulfilled.
// AnyConditions get fulfilled when at least one of its sub-conditions passes.
// AllConditions get fulfilled only when all of its sub-conditions pass.
type AnyAllConditions struct {
	// AnyConditions enable variable-based conditional rule execution. This is useful for
	// finer control of when an rule is applied. A condition can reference object data
	// using JMESPath notation.
	// Here, at least one of the conditions need to pass
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
	// are Equals, NotEquals, In, AnyIn, AllIn and NotIn, AnyNotIn, AllNotIn.
	Operator ConditionOperator `json:"operator,omitempty" yaml:"operator,omitempty"`

	// Value is the conditional value, or set of values. The values can be fixed set
	// or can be variables declared using using JMESPath.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Value apiextensions.JSON `json:"value,omitempty" yaml:"value,omitempty"`
}

// ConditionOperator is the operation performed on condition key and value.
// +kubebuilder:validation:Enum=Equals;NotEquals;In;AnyIn;AllIn;NotIn;AnyNotIn;AllNotIn;GreaterThanOrEquals;GreaterThan;LessThanOrEquals;LessThan;DurationGreaterThanOrEquals;DurationGreaterThan;DurationLessThanOrEquals;DurationLessThan
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
	// AnyIn evaluates if any of the keys are contained in the set of values.
	AnyIn ConditionOperator = "AnyIn"
	// AllIn evaluates if all the keys are contained in the set of values.
	AllIn ConditionOperator = "AllIn"
	// NotIn evaluates if the key is not contained in the set of values.
	NotIn ConditionOperator = "NotIn"
	// AnyNotIn evaluates if any of the keys are not contained in the set of values.
	AnyNotIn ConditionOperator = "AnyNotIn"
	// AllNotIn evaluates if all the keys are not contained in the set of values.
	AllNotIn ConditionOperator = "AllNotIn"
	// GreaterThanOrEquals evaluates if the key (numeric) is greater than or equal to the value (numeric).
	GreaterThanOrEquals ConditionOperator = "GreaterThanOrEquals"
	// GreaterThan evaluates if the key (numeric) is greater than the value (numeric).
	GreaterThan ConditionOperator = "GreaterThan"
	// LessThanOrEquals evaluates if the key (numeric) is less than or equal to the value (numeric).
	LessThanOrEquals ConditionOperator = "LessThanOrEquals"
	// LessThan evaluates if the key (numeric) is less than the value (numeric).
	LessThan ConditionOperator = "LessThan"
	// DurationGreaterThanOrEquals evaluates if the key (duration) is greater than or equal to the value (duration)
	DurationGreaterThanOrEquals ConditionOperator = "DurationGreaterThanOrEquals"
	// DurationGreaterThan evaluates if the key (duration) is greater than the value (duration)
	DurationGreaterThan ConditionOperator = "DurationGreaterThan"
	// DurationLessThanOrEquals evaluates if the key (duration) is less than or equal to the value (duration)
	DurationLessThanOrEquals ConditionOperator = "DurationLessThanOrEquals"
	// DurationLessThan evaluates if the key (duration) is greater than the value (duration)
	DurationLessThan ConditionOperator = "DurationLessThan"
)

// MatchResources is used to specify resource and admission review request data for
// which a policy rule is applicable.
type MatchResources struct {
	// Any allows specifying resources which will be ORed
	// +optional
	Any ResourceFilters `json:"any,omitempty" yaml:"any,omitempty"`

	// All allows specifying resources which will be ANDed
	// +optional
	All ResourceFilters `json:"all,omitempty" yaml:"all,omitempty"`

	// UserInfo contains information about the user performing the operation.
	// Specifying UserInfo directly under match is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	// Requires at least one tag to be specified when under MatchResources.
	// Specifying ResourceDescription directly under match is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ExcludeResources specifies resource and admission review request data for
// which a policy rule is not applicable.
type ExcludeResources struct {
	// Any allows specifying resources which will be ORed
	// +optional
	Any ResourceFilters `json:"any,omitempty" yaml:"any,omitempty"`

	// All allows specifying resources which will be ANDed
	// +optional
	All ResourceFilters `json:"all,omitempty" yaml:"all,omitempty"`

	// UserInfo contains information about the user performing the operation.
	// Specifying UserInfo directly under exclude is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
	// Specifying ResourceDescription directly under exclude is being deprecated.
	// Please specify under "any" or "all" instead.
	// +optional
	ResourceDescription `json:"resources,omitempty" yaml:"resources,omitempty"`
}

// ResourceFilters is a slice of ResourceFilter
type ResourceFilters []ResourceFilter

// ResourceFilter allow users to "AND" or "OR" between resources
type ResourceFilter struct {
	// UserInfo contains information about the user performing the operation.
	// +optional
	UserInfo `json:",omitempty" yaml:",omitempty"`

	// ResourceDescription contains information about the resource being created or modified.
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

	// Names are the names of the resources. Each name supports wildcard characters
	// "*" (matches zero or many characters) and "?" (at least one character).
	// NOTE: "Name" is being deprecated in favor of "Names".
	// +optional
	Names []string `json:"names,omitempty" yaml:"names,omitempty"`

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

	// ForEachMutation applies policy rule changes to nested elements.
	// +optional
	ForEachMutation []*ForEachMutation `json:"foreach,omitempty" yaml:"foreach,omitempty"`
}

// ForEachMutation applies policy rule changes to nested elements.
type ForEachMutation struct {

	// List specifies a JMESPath expression that results in one or more elements
	// to which the validation logic is applied.
	List string `json:"list,omitempty" yaml:"list,omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// AnyAllConditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyAllConditions *AnyAllConditions `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// PatchStrategicMerge is a strategic merge patch used to modify resources.
	// See https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/
	// and https://kubectl.docs.kubernetes.io/references/kustomize/patchesstrategicmerge/.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	PatchStrategicMerge apiextensions.JSON `json:"patchStrategicMerge,omitempty" yaml:"patchStrategicMerge,omitempty"`
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

	// ForEach applies policy rule changes to nested elements.
	// +optional
	ForEachValidation []*ForEachValidation `json:"foreach,omitempty" yaml:"foreach,omitempty"`

	// Pattern specifies an overlay-style pattern used to check resources.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Pattern apiextensions.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyPattern apiextensions.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Deny defines conditions used to pass or fail a validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`
}

// Deny specifies a list of conditions used to pass or fail a validation rule.
type Deny struct {
	// Multiple conditions can be declared under an `any` or `all` statement. A direct list
	// of conditions (without `any` or `all` statements) is also supported for backwards compatibility
	// but will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/validate/#deny-rules
	// +kubebuilder:validation:XPreserveUnknownFields
	AnyAllConditions apiextensions.JSON `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// ForEachValidation applies policy rule checks to nested elements.
type ForEachValidation struct {

	// List specifies a JMESPath expression that results in one or more elements
	// to which the validation logic is applied.
	List string `json:"list,omitempty" yaml:"list,omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// AnyAllConditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyAllConditions *AnyAllConditions `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`

	// Pattern specifies an overlay-style pattern used to check resources.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	Pattern apiextensions.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +kubebuilder:validation:XPreserveUnknownFields
	// +optional
	AnyPattern apiextensions.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Deny defines conditions used to pass or fail a validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`
}

// ImageVerification validates that images that match the specified pattern
// are signed with the supplied public key. Once the image is verified it is
// mutated to include the SHA digest retrieved during the registration.
type ImageVerification struct {

	// Image is the image name consisting of the registry address, repository, image, and tag.
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Key is the PEM encoded public key that the image or attestation is signed with.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`

	// Repository is an optional alternate OCI repository to use for image signatures that match this rule.
	// If specified Repository will override the default OCI image repository configured for the installation.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`

	// Attestations are optional checks for signed in-toto Statements used to verify the image.
	// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
	// OCI registry and decodes them into a list of Statement declarations.
	Attestations []*Attestation `json:"attestations,omitempty" yaml:"attestations,omitempty"`
}

// Attestation are checks for signed in-toto Statements that are used to verify the image.
// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
// OCI registry and decodes them into a list of Statements.
type Attestation struct {

	// PredicateType defines the type of Predicate contained within the Statement.
	PredicateType string `json:"predicateType,omitempty" yaml:"predicateType,omitempty"`

	// Conditions are used to verify attributes within a Predicate. If no Conditions are specified
	// the attestation check is satisfied as long there are predicates that match the predicate type.
	// +optional
	Conditions []*AnyAllConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
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
// Deprecated. Policy metrics are now available via the "/metrics" endpoint.
// See: https://kyverno.io/docs/monitoring-kyverno-with-prometheus-metrics/
type PolicyStatus struct {
	// Ready indicates if the policy is ready to serve the admission request
	Ready bool `json:"ready" yaml:"ready"`
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
