
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Policy is a specification for a Policy resource
type Policy struct {
    metav1.TypeMeta                     `json:",inline"`
    metav1.ObjectMeta                   `json:"metadata,omitempty"`
    Spec   PolicySpec                   `json:"spec"`
    Status PolicyStatus                 `json:"status"`
}

// PolicySpec is the spec for a Policy resource
type PolicySpec struct {
    FailurePolicy   *string             `json:"failurePolicy"`
    Rules           []PolicyRule        `json:"rules"`
}

// PolicyRule is policy rule that will be applied to resource
type PolicyRule struct {
    Resource    PolicyResource          `json:"resource"`          
    Patches     []PolicyPatch           `json:"patches"`
    Generators  []PolicyConfigGenerator `json:"generator"`
}

// PolicyResource describes the resource rule applied to
type PolicyResource struct {
    Kind        string                  `json:"kind"`
    Name        *string                 `json:"name"`
    Selector    *metav1.LabelSelector   `json:"selector"`
}

// PolicyPatch is TODO
type PolicyPatch struct {
    Path        string                  `json:"path"`
    Operation   string                  `json:"operation"`
    Value       int                     `json:"value"`
}

// PolicyConfigGenerator is TODO
type PolicyConfigGenerator struct {
    Name        string                  `json:"name"`
    CopyFrom    *PolicyCopyFrom         `json:"copyFrom"`
    Data        map[string]string       `json:"data"`
}

// PolicyCopyFrom is TODO
type PolicyCopyFrom struct {
    Namespace   string                  `json:"namespace"`
    Name        string                  `json:"name"`
}

// PolicyStatus is the status for a Policy resource
type PolicyStatus struct {
    Logs        []string                `json:"log"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyList is a list of Policy resources
type PolicyList struct {
    metav1.TypeMeta                     `json:",inline"`
    metav1.ListMeta                     `json:"metadata"`
    Items []Policy                      `json:"items"`
}
