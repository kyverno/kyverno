package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

type Test struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Name              string       `json:"name"`
	Policies          []string     `json:"policies,omitempty"`
	Resources         []string     `json:"resources,omitempty"`
	Variables         string       `json:"variables,omitempty"`
	UserInfo          string       `json:"userinfo,omitempty"`
	Results           []TestResult `json:"results,omitempty"`
	Values            *ValuesSpec  `json:"values,omitempty"`
}
