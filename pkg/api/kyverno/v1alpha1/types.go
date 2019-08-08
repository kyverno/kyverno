package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Policy contains rules to be applied to created resources
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              Spec         `json:"spec"`
	Status            PolicyStatus `json:"status"`
}

// Spec describes policy behavior by its rules
type Spec struct {
	Rules                   []Rule `json:"rules"`
	ValidationFailureAction string `json:"validationFailureAction"`
}

// Rule is set of mutation, validation and generation actions
// for the single resource description
type Rule struct {
	Name             string           `json:"name"`
	MatchResources   MatchResources   `json:"match"`
	ExcludeResources ExcludeResources `json:"exclude,omitempty"`
	Mutation         Mutation         `json:"mutate"`
	Validation       Validation       `json:"validate"`
	Generation       Generation       `json:"generate"`
}

//MatchResources contains resource description of the resources that the rule is to apply on
type MatchResources struct {
	ResourceDescription `json:"resources"`
}

//ExcludeResources container resource description of the resources that are to be excluded from the applying the policy rule
type ExcludeResources struct {
	ResourceDescription `json:"resources"`
}

// ResourceDescription describes the resource to which the PolicyRule will be applied.
type ResourceDescription struct {
	Kinds     []string              `json:"kinds"`
	Name      string                `json:"name"`
	Namespace string                `json:"namespace,omitempty"`
	Selector  *metav1.LabelSelector `json:"selector"`
}

// Mutation describes the way how Mutating Webhook will react on resource creation
type Mutation struct {
	Overlay interface{} `json:"overlay"`
	Patches []Patch     `json:"patches"`
}

// +k8s:deepcopy-gen=false

// Patch declares patch operation for created object according to RFC 6902
type Patch struct {
	Path      string      `json:"path"`
	Operation string      `json:"op"`
	Value     interface{} `json:"value"`
}

// Validation describes the way how Validating Webhook will check the resource on creation
type Validation struct {
	Message string      `json:"message"`
	Pattern interface{} `json:"pattern"`
}

// Generation describes which resources will be created when other resource is created
type Generation struct {
	Kind  string      `json:"kind"`
	Name  string      `json:"name"`
	Data  interface{} `json:"data"`
	Clone CloneFrom   `json:"clone"`
}

// CloneFrom - location of a Secret or a ConfigMap
// which will be used as source when applying 'generate'
type CloneFrom struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

//PolicyStatus provides status for violations
type PolicyStatus struct {
	Violations int `json:"violations"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyList is a list of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Policy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyViolation stores the information regarinding the resources for which a policy failed to apply
type PolicyViolation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PolicyViolationSpec `json:"spec"`
	Status            string              `json:"status"`
}

// PolicyViolationSpec describes policy behavior by its rules
type PolicyViolationSpec struct {
	Policy        string `json:"policy"`
	ResourceSpec  `json:"resource"`
	ViolatedRules []ViolatedRule `json:"rules"`
}

// ResourceSpec information to identify the resource
type ResourceSpec struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

type ViolatedRule struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyViolationList is a list of Policy Violation
type PolicyViolationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyViolation `json:"items"`
}
