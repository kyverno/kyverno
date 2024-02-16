package v2

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// WebhookConfiguration specifies the configuration for Kubernetes admission webhookconfiguration.
type WebhookConfiguration struct {
	// MatchCondition configures admission webhook matchConditions.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty" yaml:"matchConditions,omitempty"`
}

// Validation defines checks to be performed on matching resources.
type Validation struct {
	// ValidationFailureAction defines if a validation policy rule violation should block
	// the admission review request (enforce), or allow (audit) the admission review request
	// and report an error in a policy report. Optional.
	// Allowed values are audit or enforce. The default value is "Audit".
	// +optional
	// +kubebuilder:validation:Enum=audit;enforce;Audit;Enforce
	// +kubebuilder:default=Audit
	ValidationFailureAction kyvernov1.ValidationFailureAction `json:"validationFailureAction,omitempty" yaml:"validationFailureAction,omitempty"`

	// ValidationFailureActionOverrides is a Cluster Policy attribute that specifies ValidationFailureAction
	// namespace-wise. It overrides ValidationFailureAction for the specified namespaces.
	// +optional
	ValidationFailureActionOverrides []kyvernov1.ValidationFailureActionOverride `json:"validationFailureActionOverrides,omitempty" yaml:"validationFailureActionOverrides,omitempty"`

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

// Mutation defines how resource are modified.
type Mutation struct {
	// MutateExistingOnPolicyUpdate controls if a mutateExisting policy is applied on policy events.
	// Default value is "false".
	// +optional
	MutateExistingOnPolicyUpdate bool `json:"mutateExistingOnPolicyUpdate,omitempty" yaml:"mutateExistingOnPolicyUpdate,omitempty"`

	// Targets defines the target resources to be mutated.
	// +optional
	Targets []TargetResourceSpec `json:"targets,omitempty" yaml:"targets,omitempty"`

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
	ForEachMutation []kyvernov1.ForEachMutation `json:"foreach,omitempty" yaml:"foreach,omitempty"`
}

// TargetResourceSpec defines targets for mutating existing resources.
type TargetResourceSpec struct {
	// ResourceSpec contains the target resources to load when mutating existing resources.
	kyvernov1.ResourceSpec `json:",omitempty" yaml:",omitempty"`

	// Context defines variables and data sources that can be used during rule execution.
	// +optional
	Context []kyvernov1.ContextEntry `json:"context,omitempty" yaml:"context,omitempty"`

	// Preconditions are used to determine if a policy rule should be applied by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// See: https://kyverno.io/docs/writing-policies/preconditions/
	// +optional
	RawAnyAllConditions *AnyAllConditions `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`
}

// Generation defines how new resources should be created and managed.
type Generation struct {
	// ResourceSpec contains information to select the resource.
	kyvernov1.ResourceSpec `json:",omitempty" yaml:",omitempty"`

	// GenerateExisting controls whether to trigger generate rule in existing resources
	// If is set to "true" generate rule will be triggered and applied to existing matched resources.
	// Defaults to "false" if not specified.
	// +optional
	GenerateExisting bool `json:"generateExisting,omitempty" yaml:"generateExisting,omitempty"`

	// Synchronize controls if generated resources should be kept in-sync with their source resource.
	// If Synchronize is set to "true" changes to generated resources will be overwritten with resource
	// data from Data or the resource specified in the Clone declaration.
	// Optional. Defaults to "false" if not specified.
	// +optional
	Synchronize bool `json:"synchronize,omitempty" yaml:"synchronize,omitempty"`

	// OrphanDownstreamOnPolicyDelete controls whether generated resources should be deleted when the rule that generated
	// them is deleted with synchronization enabled. This option is only applicable to generate rules of the data type.
	// See https://kyverno.io/docs/writing-policies/generate/#data-examples.
	// Defaults to "false" if not specified.
	// +optional
	OrphanDownstreamOnPolicyDelete bool `json:"orphanDownstreamOnPolicyDelete,omitempty" yaml:"orphanDownstreamOnPolicyDelete,omitempty"`

	// Data provides the resource declaration used to populate each generated resource.
	// At most one of Data or Clone must be specified. If neither are provided, the generated
	// resource will be created with default data only.
	// +optional
	RawData *apiextv1.JSON `json:"data,omitempty" yaml:"data,omitempty"`

	// Clone specifies the source resource used to populate each generated resource.
	// At most one of Data or Clone can be specified. If neither are provided, the generated
	// resource will be created with default data only.
	// +optional
	Clone kyvernov1.CloneFrom `json:"clone,omitempty" yaml:"clone,omitempty"`

	// CloneList specifies the list of source resource used to populate each generated resource.
	// +optional
	CloneList kyvernov1.CloneList `json:"cloneList,omitempty" yaml:"cloneList,omitempty"`
}

func (g *Generation) Validate(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	if namespaced {
		if err := g.validateNamespacedTargetsScope(clusterResources, policyNamespace); err != nil {
			errs = append(errs, field.Forbidden(path.Child("generate").Child("namespace"), fmt.Sprintf("target resource scope mismatched: %v ", err)))
		}
	}

	if g.GetKind() != "" {
		if !clusterResources.Has(g.GetAPIVersion() + "/" + g.GetKind()) {
			if g.GetNamespace() == "" {
				errs = append(errs, field.Forbidden(path.Child("generate").Child("namespace"), "target namespace must be set for a namespaced resource"))
			}
		} else {
			if g.GetNamespace() != "" {
				errs = append(errs, field.Forbidden(path.Child("generate").Child("namespace"), "target namespace must not be set for a cluster-wide resource"))
			}
		}
	}

	generateType, _, _ := g.GetTypeAndSyncAndOrphanDownstream()
	if generateType == kyvernov1.Data {
		return errs
	}

	newGeneration := Generation{
		ResourceSpec: kyvernov1.ResourceSpec{
			Kind:       g.ResourceSpec.GetKind(),
			APIVersion: g.ResourceSpec.GetAPIVersion(),
		},
		Clone:     g.Clone,
		CloneList: g.CloneList,
	}

	if err := regex.ObjectHasVariables(newGeneration); err != nil {
		errs = append(errs, field.Forbidden(path.Child("generate").Child("clone/cloneList"), "Generation Rule Clone/CloneList should not have variables"))
	}

	if len(g.CloneList.Kinds) == 0 {
		if g.Kind == "" {
			errs = append(errs, field.Forbidden(path.Child("generate").Child("kind"), "kind can not be empty"))
		}
		if g.Name == "" {
			errs = append(errs, field.Forbidden(path.Child("generate").Child("name"), "name can not be empty"))
		}
	}

	errs = append(errs, g.ValidateCloneList(path.Child("generate"), namespaced, policyNamespace, clusterResources)...)
	return errs
}

func (g *Generation) ValidateCloneList(path *field.Path, namespaced bool, policyNamespace string, clusterResources sets.Set[string]) (errs field.ErrorList) {
	if len(g.CloneList.Kinds) == 0 {
		return nil
	}

	if namespaced {
		for _, kind := range g.CloneList.Kinds {
			if clusterResources.Has(kind) {
				errs = append(errs, field.Forbidden(path.Child("cloneList").Child("kinds"), fmt.Sprintf("the source in cloneList must be a namespaced resource: %v", kind)))
			}
			if g.CloneList.Namespace != policyNamespace {
				errs = append(errs, field.Forbidden(path.Child("cloneList").Child("namespace"), fmt.Sprintf("a namespaced policy cannot clone resources from other namespace, expected: %v, received: %v", policyNamespace, g.CloneList.Namespace)))
			}
		}
	}

	clusterScope := clusterResources.Has(g.CloneList.Kinds[0])
	for _, gvk := range g.CloneList.Kinds[1:] {
		if clusterScope != clusterResources.Has(gvk) {
			errs = append(errs, field.Forbidden(path.Child("cloneList").Child("kinds"), "mixed scope of target resources is forbidden"))
			break
		}
		clusterScope = clusterScope && clusterResources.Has(gvk)
	}

	if !clusterScope {
		if g.CloneList.Namespace == "" {
			errs = append(errs, field.Forbidden(path.Child("cloneList").Child("namespace"), "namespace is required for namespaced target resources"))
		}
	} else if clusterScope && !namespaced {
		if g.CloneList.Namespace != "" {
			errs = append(errs, field.Forbidden(path.Child("cloneList").Child("namespace"), "namespace is forbidden for cluster-wide target resources"))
		}
	}
	return errs
}

func (g *Generation) GetData() apiextensions.JSON {
	return kyvernov1.FromJSON(g.RawData)
}

func (g *Generation) SetData(in apiextensions.JSON) {
	g.RawData = kyvernov1.ToJSON(in)
}

func (g *Generation) validateNamespacedTargetsScope(clusterResources sets.Set[string], policyNamespace string) error {
	target := g.ResourceSpec
	if clusterResources.Has(target.GetAPIVersion() + "/" + target.GetKind()) {
		return fmt.Errorf("the target must be a namespaced resource: %v/%v", target.GetAPIVersion(), target.GetKind())
	}

	if g.GetNamespace() != policyNamespace {
		return fmt.Errorf("a namespaced policy cannot generate resources in other namespaces, expected: %v, received: %v", policyNamespace, g.GetNamespace())
	}

	if g.Clone.Name != "" {
		if g.Clone.Namespace != policyNamespace {
			return fmt.Errorf("a namespaced policy cannot clone resources from other namespaces, expected: %v, received: %v", policyNamespace, g.Clone.Namespace)
		}
	}
	return nil
}

func (g *Generation) GetTypeAndSyncAndOrphanDownstream() (kyvernov1.GenerateType, bool, bool) {
	if g.RawData != nil {
		return kyvernov1.Data, g.Synchronize, g.OrphanDownstreamOnPolicyDelete
	}
	return kyvernov1.Clone, g.Synchronize, g.OrphanDownstreamOnPolicyDelete
}
