package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// An example of the YAML representation of this structure is here:
// <project_root>/crd/policy-example.yaml
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              PolicySpec   `json:"spec"`
	Status            PolicyStatus `json:"status"`
}

// Specification of the Policy.
// failurePolicy can have values "continueOnError" and "stopOnError" (default).
type PolicySpec struct {
	FailurePolicy *string      `json:"failurePolicy"`
	Rules         []PolicyRule `json:"rules"`
}

// The rule of mutation for the single resource definition.
// Details are listed in the description of each of the substructures.
type PolicyRule struct {
	Name               string                 `json:"name"`
	Resource           PolicyResource         `json:"resource"`
	Patches            []PolicyPatch          `json:"patch,omitempty"`
	ConfigMapGenerator *PolicyConfigGenerator `json:"configMapGenerator,omitempty"`
	SecretGenerator    *PolicyConfigGenerator `json:"secretGenerator,omitempty"`
}

// Describes the resource to which the PolicyRule will apply.
// Either the name or selector must be specified.
// IMPORTANT: If neither is specified, the policy rule will not apply (TBD).
type PolicyResource struct {
	Kind     string                `json:"kind"`
	Name     *string               `json:"name"`
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// +k8s:deepcopy-gen=false

// PolicyPatch declares patch operation for created object according to the JSONPatch spec:
// http://jsonpatch.com/
type PolicyPatch struct {
	Path      string      `json:"path"`
	Operation string      `json:"op"`
	Value     interface{} `json:"value"`
}

func (in *PolicyPatch) DeepCopyInto(out *PolicyPatch) {
	if out != nil {
		*out = *in
	}
}

// The declaration for a Secret or a ConfigMap, which will be created in the new namespace.
// Can be applied only when PolicyRule.Resource.Kind is "Namespace".
type PolicyConfigGenerator struct {
	Name     string            `json:"name"`
	CopyFrom *PolicyCopyFrom   `json:"copyFrom"`
	Data     map[string]string `json:"data"`
}

// Location of a Secret or a ConfigMap which will be used as source when applying PolicyConfigGenerator
type PolicyCopyFrom struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Contains logs about policy application
type PolicyStatus struct {
	Logs       []string    `json:"log"`
	Violations []Violation `json:"violations,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// List of Policy resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Policy `json:"items"`
}

// Violation for the policy
type Violation struct {
	Kind     string `json:"kind,omitempty"`
	Resource string `json:"resource,omitempty"`
	Rule     string `json:"rule,omitempty"`
}
