package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CleanupPolicyInterface abstracts the concrete policy type (Policy vs ClusterPolicy)
// +kubebuilder:object:generate=false
type CleanupPolicyInterface interface {
	metav1.Object
	GetSpec() *CleanupPolicySpec
	GetStatus() *CleanupPolicyStatus
	Validate(sets.Set[string]) field.ErrorList
	GetKind() string
	GetAPIVersion() string
}
