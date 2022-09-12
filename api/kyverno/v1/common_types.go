package v1

import (
	"encoding/json"

	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/pod-security-admission/api"
)

// FailurePolicyType specifies a failure policy that defines how unrecognized errors from the admission endpoint are handled.
// +kubebuilder:validation:Enum=Ignore;Fail
type FailurePolicyType string

const (
	// Ignore means that an error calling the webhook is ignored.
	Ignore FailurePolicyType = "Ignore"
	// Fail means that an error calling the webhook causes the admission to fail.
	Fail FailurePolicyType = "Fail"
)

// ApplyRulesType controls whether processing stops after one rule is applied or all rules are applied.
// +kubebuilder:validation:Enum=All;One
type ApplyRulesType string

const (
	// AllMatchingRules applies all rules in a policy that match.
	ApplyAll ApplyRulesType = "All"
	// FirstMatchingRule applies only the first matching rule in the policy.
	ApplyOne ApplyRulesType = "One"
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

	// ImageRegistry defines requests to an OCI/Docker V2 registry to fetch image
	// details.
	ImageRegistry *ImageRegistry `json:"imageRegistry,omitempty" yaml:"imageRegistry,omitempty"`

	// Variable defines an arbitrary JMESPath context variable that can be defined inline.
	Variable *Variable `json:"variable,omitempty" yaml:"variable,omitempty"`
}

// Variable defines an arbitrary JMESPath context variable that can be defined inline.
type Variable struct {
	// Value is any arbitrary JSON object representable in YAML or JSON form.
	// +optional
	Value *apiextv1.JSON `json:"value,omitempty" yaml:"value,omitempty"`

	// JMESPath is an optional JMESPath Expression that can be used to
	// transform the variable.
	// +optional
	JMESPath string `json:"jmesPath,omitempty" yaml:"jmesPath,omitempty"`

	// Default is an optional arbitrary JSON object that the variable may take if the JMESPath
	// expression evaluates to nil
	// +optional
	Default *apiextv1.JSON `json:"default,omitempty" yaml:"default,omitempty"`
}

// ImageRegistry defines requests to an OCI/Docker V2 registry to fetch image
// details.
type ImageRegistry struct {
	// Reference is image reference to a container image in the registry.
	// Example: ghcr.io/kyverno/kyverno:latest
	Reference string `json:"reference" yaml:"reference"`

	// JMESPath is an optional JSON Match Expression that can be used to
	// transform the ImageData struct returned as a result of processing
	// the image reference.
	// +optional
	JMESPath string `json:"jmesPath,omitempty" yaml:"jmesPath,omitempty"`
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
}

func (c *Condition) GetKey() apiextensions.JSON {
	return FromJSON(c.RawKey)
}

func (c *Condition) SetKey(in apiextensions.JSON) {
	c.RawKey = ToJSON(in)
}

func (c *Condition) GetValue() apiextensions.JSON {
	return FromJSON(c.RawValue)
}

func (c *Condition) SetValue(in apiextensions.JSON) {
	c.RawValue = ToJSON(in)
}

// ConditionOperator is the operation performed on condition key and value.
// +kubebuilder:validation:Enum=Equals;NotEquals;In;AnyIn;AllIn;NotIn;AnyNotIn;AllNotIn;GreaterThanOrEquals;GreaterThan;LessThanOrEquals;LessThan;DurationGreaterThanOrEquals;DurationGreaterThan;DurationLessThanOrEquals;DurationLessThan
type ConditionOperator string

// ConditionOperators stores all the valid ConditionOperator types as key-value pairs.
//
// "Equal" evaluates if the key is equal to the value. (Deprecated; Use Equals instead)
// "Equals" evaluates if the key is equal to the value.
// "NotEqual" evaluates if the key is not equal to the value. (Deprecated; Use NotEquals instead)
// "NotEquals" evaluates if the key is not equal to the value.
// "In" evaluates if the key is contained in the set of values.
// "AnyIn" evaluates if any of the keys are contained in the set of values.
// "AllIn" evaluates if all the keys are contained in the set of values.
// "NotIn" evaluates if the key is not contained in the set of values.
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
	"Equal":                       ConditionOperator("Equal"),
	"Equals":                      ConditionOperator("Equals"),
	"NotEqual":                    ConditionOperator("NotEqual"),
	"NotEquals":                   ConditionOperator("NotEquals"),
	"In":                          ConditionOperator("In"),
	"AnyIn":                       ConditionOperator("AnyIn"),
	"AllIn":                       ConditionOperator("AllIn"),
	"NotIn":                       ConditionOperator("NotIn"),
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

// Mutation defines how resource are modified.
type Mutation struct {
	// Targets defines the target resources to be mutated.
	// +optional
	Targets []ResourceSpec `json:"targets,omitempty" yaml:"targets,omitempty"`

	// PatchStrategicMerge is a strategic merge patch used to modify resources.
	// See https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/
	// and https://kubectl.docs.kubernetes.io/references/kustomize/patchesstrategicmerge/.
	// +optional
	RawPatchStrategicMerge *apiextv1.JSON `json:"patchStrategicMerge,omitempty" yaml:"patchStrategicMerge,omitempty"`

	// PatchesJSON6902 is a list of RFC 6902 JSON Patch declarations used to modify resources.
	// See https://tools.ietf.org/html/rfc6902 and https://kubectl.docs.kubernetes.io/references/kustomize/patchesjson6902/.
	// +optional
	PatchesJSON6902 string `json:"patchesJson6902,omitempty" yaml:"patchesJson6902,omitempty"`

	// ForEach applies mutation rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
	// +optional
	ForEachMutation []ForEachMutation `json:"foreach,omitempty" yaml:"foreach,omitempty"`
}

func (m *Mutation) GetPatchStrategicMerge() apiextensions.JSON {
	return FromJSON(m.RawPatchStrategicMerge)
}

func (m *Mutation) SetPatchStrategicMerge(in apiextensions.JSON) {
	m.RawPatchStrategicMerge = ToJSON(in)
}

// ForEach applies mutation rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
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
	// +optional
	RawPatchStrategicMerge *apiextv1.JSON `json:"patchStrategicMerge,omitempty" yaml:"patchStrategicMerge,omitempty"`

	// PatchesJSON6902 is a list of RFC 6902 JSON Patch declarations used to modify resources.
	// See https://tools.ietf.org/html/rfc6902 and https://kubectl.docs.kubernetes.io/references/kustomize/patchesjson6902/.
	// +optional
	PatchesJSON6902 string `json:"patchesJson6902,omitempty" yaml:"patchesJson6902,omitempty"`
}

func (m *ForEachMutation) GetPatchStrategicMerge() apiextensions.JSON {
	return FromJSON(m.RawPatchStrategicMerge)
}

func (m *ForEachMutation) SetPatchStrategicMerge(in apiextensions.JSON) {
	m.RawPatchStrategicMerge = ToJSON(in)
}

// Validation defines checks to be performed on matching resources.
type Validation struct {
	// Message specifies a custom message to be displayed on failure.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Manifest specifies conditions for manifest verification
	// +optional
	Manifests *Manifests `json:"manifests,omitempty" yaml:"manifests,omitempty"`

	// ForEach applies validate rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
	// +optional
	ForEachValidation []ForEachValidation `json:"foreach,omitempty" yaml:"foreach,omitempty"`

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
	PodSecurity *PodSecurity `json:"podSecurity,omitempty" yaml:"podSecurity,omitempty"`
}

// PodSecurity applies exemptions for Kubernetes Pod Security admission
// by specifying exclusions for Pod Security Standards controls.
type PodSecurity struct {
	// Level defines the Pod Security Standard level to be applied to workloads.
	// Allowed values are privileged, baseline, and restricted.
	// +kubebuilder:validation:Enum=privileged;baseline;restricted
	Level api.Level `json:"level,omitempty" yaml:"level,omitempty"`

	// Version defines the Pod Security Standard versions that Kubernetes supports.
	// Allowed values are v1.19, v1.20, v1.21, v1.22, v1.23, v1.24, v1.25, latest. Defaults to latest.
	// +kubebuilder:validation:Enum=v1.19;v1.20;v1.21;v1.22;v1.23;v1.24;v1.25;latest
	// +optional
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Exclude specifies the Pod Security Standard controls to be excluded.
	Exclude []PodSecurityStandard `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// PodSecurityStandard specifies the Pod Security Standard controls to be excluded.
type PodSecurityStandard struct {
	// ControlName specifies the name of the Pod Security Standard control.
	// See: https://kubernetes.io/docs/concepts/security/pod-security-standards/
	ControlName string `json:"controlName" yaml:"controlName"`

	// Images is a list of matching image patterns.
	// Each image is the image name consisting of the registry address, repository, image, and tag.
	// +optional
	Images []string `json:"images,omitempty" yaml:"images,omitempty"`

	// RestrictedField selects the field for the given Pod Security Standard control.
	// When not set, all restricted fields for the control are selected.
	// +optional
	RestrictedField string `json:"restrictedField,omitempty" yaml:"restrictedField,omitempty"`

	// Values defines the allowed values that can be excluded.
	// +optional
	Values []string `json:"values,omitempty" yaml:"values,omitempty"`
}

// DeserializeAnyPattern deserialize apiextensions.JSON to []interface{}
func (in *Validation) DeserializeAnyPattern() ([]interface{}, error) {
	anyPattern := in.GetAnyPattern()
	if anyPattern == nil {
		return nil, nil
	}
	res, nil := deserializePattern(anyPattern)
	return res, nil
}

func deserializePattern(pattern apiextensions.JSON) ([]interface{}, error) {
	anyPattern, err := json.Marshal(pattern)
	if err != nil {
		return nil, err
	}

	var res []interface{}
	if err := json.Unmarshal(anyPattern, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (v *Validation) GetPattern() apiextensions.JSON {
	return FromJSON(v.RawPattern)
}

func (v *Validation) SetPattern(in apiextensions.JSON) {
	v.RawPattern = ToJSON(in)
}

func (v *Validation) GetAnyPattern() apiextensions.JSON {
	return FromJSON(v.RawAnyPattern)
}

func (v *Validation) SetAnyPattern(in apiextensions.JSON) {
	v.RawAnyPattern = ToJSON(in)
}

// Deny specifies a list of conditions used to pass or fail a validation rule.
type Deny struct {
	// Multiple conditions can be declared under an `any` or `all` statement. A direct list
	// of conditions (without `any` or `all` statements) is also supported for backwards compatibility
	// but will be deprecated in the next major release.
	// See: https://kyverno.io/docs/writing-policies/validate/#deny-rules
	RawAnyAllConditions *apiextv1.JSON `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

func (d *Deny) GetAnyAllConditions() apiextensions.JSON {
	return FromJSON(d.RawAnyAllConditions)
}

func (d *Deny) SetAnyAllConditions(in apiextensions.JSON) {
	d.RawAnyAllConditions = ToJSON(in)
}

// ForEach applies validate rules to a list of sub-elements by creating a context for each entry in the list and looping over it to apply the specified logic.
type ForEachValidation struct {
	// List specifies a JMESPath expression that results in one or more elements
	// to which the validation logic is applied.
	List string `json:"list,omitempty" yaml:"list,omitempty"`

	// ElementScope specifies whether to use the current list element as the scope for validation. Defaults to "true" if not specified.
	// When set to "false", "request.object" is used as the validation scope within the foreach
	// block to allow referencing other elements in the subtree.
	// +optional
	ElementScope *bool `json:"elementScope,omitempty" yaml:"elementScope,omitempty"`

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
	// +optional
	RawPattern *apiextv1.JSON `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// AnyPattern specifies list of validation patterns. At least one of the patterns
	// must be satisfied for the validation rule to succeed.
	// +optional
	RawAnyPattern *apiextv1.JSON `json:"anyPattern,omitempty" yaml:"anyPattern,omitempty"`

	// Deny defines conditions used to pass or fail a validation rule.
	// +optional
	Deny *Deny `json:"deny,omitempty" yaml:"deny,omitempty"`
}

func (v *ForEachValidation) GetPattern() apiextensions.JSON {
	return FromJSON(v.RawPattern)
}

func (v *ForEachValidation) SetPattern(in apiextensions.JSON) {
	v.RawPattern = ToJSON(in)
}

func (v *ForEachValidation) GetAnyPattern() apiextensions.JSON {
	return FromJSON(v.RawAnyPattern)
}

func (v *ForEachValidation) SetAnyPattern(in apiextensions.JSON) {
	v.RawAnyPattern = ToJSON(in)
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
	// +optional
	RawData *apiextv1.JSON `json:"data,omitempty" yaml:"data,omitempty"`

	// Clone specifies the source resource used to populate each generated resource.
	// At most one of Data or Clone can be specified. If neither are provided, the generated
	// resource will be created with default data only.
	// +optional
	Clone CloneFrom `json:"clone,omitempty" yaml:"clone,omitempty"`

	// CloneList specifies the list of source resource used to populate each generated resource.
	// +optional
	CloneList CloneList `json:"cloneList,omitempty" yaml:"cloneList,omitempty"`
}

type CloneList struct {
	// Namespace specifies source resource namespace.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	// Kinds is a list of resource kinds.
	Kinds []string `json:"kinds,omitempty" yaml:"kinds,omitempty"`
}

func (g *Generation) GetData() apiextensions.JSON {
	return FromJSON(g.RawData)
}

func (g *Generation) SetData(in apiextensions.JSON) {
	g.RawData = ToJSON(in)
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

type Manifests struct {
	// Attestors specified the required attestors (i.e. authorities)
	// +kubebuilder:validation:Optional
	Attestors []AttestorSet `json:"attestors,omitempty" yaml:"attestors,omitempty"`

	// AnnotationDomain is custom domain of annotation for message and signature. Default is "cosign.sigstore.dev".
	// +optional
	AnnotationDomain string `json:"annotationDomain,omitempty" yaml:"annotationDomain,omitempty"`

	// Fields which will be ignored while comparing manifests.
	// +optional
	IgnoreFields IgnoreFieldList `json:"ignoreFields,omitempty" yaml:"ignoreFields,omitempty"`

	// DryRun configuration
	// +optional
	DryRunOption DryRunOption `json:"dryRun,omitempty" yaml:"dryRun,omitempty"`

	// Repository is an optional alternate OCI repository to use for resource bundle reference.
	// The repository can be overridden per Attestor or Attestation.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
}

// DryRunOption is a configuration for dryrun.
// If enable is set to "true", manifest verification performs "dryrun & compare"
// which provides robust matching against changes by defaults and other admission controllers.
// Dryrun requires additional permissions. See config/dryrun/dryrun_rbac.yaml
type DryRunOption struct {
	Enable    bool   `json:"enable,omitempty" yaml:"enable,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

type IgnoreFieldList []ObjectFieldBinding

type ObjectFieldBinding k8smanifest.ObjectFieldBinding
