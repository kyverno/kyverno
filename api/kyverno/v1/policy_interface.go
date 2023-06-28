package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// PolicyInterface abstracts the concrete policy type (Policy vs ClusterPolicy)
// +kubebuilder:object:generate=false
type PolicyInterface interface {
	metav1.Object
	AdmissionProcessingEnabled() bool
	BackgroundProcessingEnabled() bool
	IsNamespaced() bool
	GetSpec() *Spec
	GetStatus() *PolicyStatus
	Validate(sets.Set[string]) field.ErrorList
	GetKind() string
	CreateDeepCopy() PolicyInterface
	IsReady() bool
	ValidateSchema() bool
}
