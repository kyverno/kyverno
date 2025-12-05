package v1beta1

import (
	"time"

	"github.com/aptible/supercronic/cronexpr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DeletingPolicyKind           = "DeletingPolicy"
	NamespacedDeletingPolicyKind = "NamespacedDeletingPolicy"
)

type (
	DeletingPolicySpec   = v1alpha1.DeletingPolicySpec
	DeletingPolicyStatus = v1alpha1.DeletingPolicyStatus
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=deletingpolicies,scope="Cluster",shortName=dpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DeletingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeletingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status DeletingPolicyStatus `json:"status,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=ndpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:storageversion

type NamespacedDeletingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DeletingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status DeletingPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeletingPolicyList is a list of DeletingPolicy instances
type DeletingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DeletingPolicy `json:"items"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NamespacedDeletingPolicyList is a list of NamespacedDeletingPolicy instances
type NamespacedDeletingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []NamespacedDeletingPolicy `json:"items"`
}

// DeletingPolicyLike captures the common behavior shared by deleting policies regardless of scope.
// +k8s:deepcopy-gen=false
type DeletingPolicyLike interface {
	metav1.Object
	runtime.Object
	GetDeletingPolicySpec() *DeletingPolicySpec
	GetKind() string
	GetExecutionTime() (*time.Time, error)
	GetNextExecutionTime(time.Time) (*time.Time, error)
}

// GetExecutionTime returns the execution time of the policy
func (p *DeletingPolicy) GetExecutionTime() (*time.Time, error) {
	return computeDeletingPolicyExecutionTime(p.Spec.Schedule, p.Status.LastExecutionTime, p.GetCreationTimestamp().Time)
}

// GetNextExecutionTime returns the next execution time of the policy
func (p *DeletingPolicy) GetNextExecutionTime(time time.Time) (*time.Time, error) {
	return computeDeletingPolicyNextExecutionTime(p.Spec.Schedule, time)
}

func (p *DeletingPolicy) GetKind() string {
	return DeletingPolicyKind
}

func (p *DeletingPolicy) GetDeletingPolicySpec() *DeletingPolicySpec {
	if p == nil {
		return nil
	}
	return &p.Spec
}

// GetExecutionTime returns the execution time of the namespaced policy
func (p *NamespacedDeletingPolicy) GetExecutionTime() (*time.Time, error) {
	return computeDeletingPolicyExecutionTime(p.Spec.Schedule, p.Status.LastExecutionTime, p.GetCreationTimestamp().Time)
}

// GetNextExecutionTime returns the next execution time of the namespaced policy
func (p *NamespacedDeletingPolicy) GetNextExecutionTime(time time.Time) (*time.Time, error) {
	return computeDeletingPolicyNextExecutionTime(p.Spec.Schedule, time)
}

func (p *NamespacedDeletingPolicy) GetKind() string {
	return NamespacedDeletingPolicyKind
}

func (p *NamespacedDeletingPolicy) GetDeletingPolicySpec() *DeletingPolicySpec {
	if p == nil {
		return nil
	}
	return &p.Spec
}

func computeDeletingPolicyExecutionTime(schedule string, lastExecution metav1.Time, creationTime time.Time) (*time.Time, error) {
	referenceTime := creationTime
	if !lastExecution.IsZero() {
		referenceTime = lastExecution.Time
	}
	return computeDeletingPolicyNextExecutionTime(schedule, referenceTime)
}

func computeDeletingPolicyNextExecutionTime(schedule string, t time.Time) (*time.Time, error) {
	cronExpr, err := cronexpr.Parse(schedule)
	if err != nil {
		return nil, err
	}
	nextExecutionTime := cronExpr.Next(t)
	return &nextExecutionTime, nil
}
