package values

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Policy struct {
	Name      string     `json:"name"`
	Resources []Resource `json:"resources"`
	Rules     []Rule     `json:"rules"`
}

type Rule struct {
	Name          string                   `json:"name"`
	Values        map[string]interface{}   `json:"values"`
	ForeachValues map[string][]interface{} `json:"foreachValues"`
}

type Values struct {
	Policies           []Policy            `json:"policies"`
	GlobalValues       map[string]string   `json:"globalValues"`
	NamespaceSelectors []NamespaceSelector `json:"namespaceSelector"`
	Subresources       []Subresource       `json:"subresources"`
}

type Resource struct {
	Name   string                 `json:"name"`
	Values map[string]interface{} `json:"values"`
}

type Subresource struct {
	APIResource    metav1.APIResource `json:"subresource"`
	ParentResource metav1.APIResource `json:"parentResource"`
}

type NamespaceSelector struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}
