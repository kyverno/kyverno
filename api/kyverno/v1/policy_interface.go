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
	BackgroundProcessingEnabled() bool
	HasAutoGenAnnotation() bool
	IsNamespaced() bool
	GetSpec() *Spec
	Validate(sets.String) field.ErrorList
	GetKind() string
	CreateDeepCopy() PolicyInterface
	IsReady() bool
}
