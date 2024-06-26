package v2

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// CleanupPolicyInterface abstracts the concrete policy type (CleanupPolicy vs ClusterCleanupPolicy)
// +kubebuilder:object:generate=false
type CleanupPolicyInterface interface {
	metav1.Object
	IsNamespaced() bool
	GetSpec() *CleanupPolicySpec
	GetStatus() *CleanupPolicyStatus
	GetExecutionTime() (*time.Time, error)
	GetNextExecutionTime(time.Time) (*time.Time, error)
	Validate(sets.Set[string]) field.ErrorList
	GetKind() string
	GetAPIVersion() string
}
