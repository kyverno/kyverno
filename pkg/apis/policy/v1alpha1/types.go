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
	Spec              Spec   `json:"spec"`
	Status            Status `json:"status"`
}

// Spec describes policy behavior by its rules
type Spec struct {
	Rules []Rule `json:"rules"`
}

// Rule is set of mutation, validation and generation actions
// for the single resource description
type Rule struct {
	Name                string `json:"name"`
	ResourceDescription `json:"resource"`
	Mutation            *Mutation   `json:"mutate"`
	Validation          *Validation `json:"validate"`
	Generation          *Generation `json:"generate"`
}

// ResourceDescription describes the resource to which the PolicyRule will be applied.
type ResourceDescription struct {
	Kinds     []string              `json:"kinds"`
	Name      *string               `json:"name"`
	Namespace *string               `json:"namespace,omitempty"`
	Selector  *metav1.LabelSelector `json:"selector"`
}

// Mutation describes the way how Mutating Webhook will react on resource creation
type Mutation struct {
	Overlay *interface{} `json:"overlay"`
	Patches []Patch      `json:"patches"`
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
	Message *string     `json:"message"`
	Pattern interface{} `json:"pattern"`
}

// Generation describes which resources will be created when other resource is created
type Generation struct {
	Kind  string      `json:"kind"`
	Name  string      `json:"name"`
	Data  interface{} `json:"data"`
	Clone *CloneFrom  `json:"clone"`
}

// CloneFrom - location of a Secret or a ConfigMap
// which will be used as source when applying 'generate'
type CloneFrom struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Status contains violations for existing resources
type Status struct {
	// Violations map[kind/namespace/resource]Violation
	Violations map[string]Violation `json:"violations,omitempty"`
}

// Violation for the policy
type Violation struct {
	Kind      string   `json:"kind,omitempty"`
	Name      string   `json:"name,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	Rules     []string `json:"rules"`
	Reason    string   `json:"reason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyList is a list of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Policy `json:"items"`
}
