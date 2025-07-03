package v1alpha1

import (
	"time"

	"github.com/aptible/supercronic/cronexpr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeletingPolicyList is a list of DeletingPolicy instances
type DeletingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DeletingPolicy `json:"items"`
}

// DeletingPolicySpec is the specification of the desired behavior of the DeletingPolicy.
type DeletingPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to validate.
	// The AdmissionPolicy cares about a request if it matches _all_ Constraints.
	// Required.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints,omitempty"`

	// Conditions is a list of conditions that must be met for a resource to be deleted.
	// Conditions filter resources that have already been matched by the match constraints,
	// namespaceSelector, and objectSelector. An empty list of conditions matches all resources.
	// There are a maximum of 64 conditions allowed.
	//
	// The exact matching logic is (in order):
	//   1. If ANY condition evaluates to FALSE, the policy is skipped.
	//   2. If ALL conditions evaluate to TRUE, the policy is executed.
	//
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	Conditions []admissionregistrationv1.MatchCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// Variables contain definitions of variables that can be used in composition of other expressions.
	// Each variable is defined as a named CEL expression.
	// The variables defined here will be available under `variables` in other expressions of the policy
	// except MatchConditions because MatchConditions are evaluated before the rest of the policy.
	//
	// The expression of a variable can refer to other variables defined earlier in the list but not those after.
	// Thus, Variables must be sorted by the order of first appearance and acyclic.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	Variables []admissionregistrationv1.Variable `json:"variables,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// The schedule in Cron format
	// Required.
	Schedule string `json:"schedule"`

	// DeletionPropagationPolicy defines how resources will be deleted (Foreground, Background, Orphan).
	// +optional
	// +kubebuilder:validation:Enum=Foreground;Background;Orphan
	DeletionPropagationPolicy *metav1.DeletionPropagation `json:"deletionPropagationPolicy,omitempty"`
}

type DeletingPolicyStatus struct {
	// +optional
	ConditionStatus   ConditionStatus `json:"conditionStatus,omitempty"`
	LastExecutionTime metav1.Time     `json:"lastExecutionTime,omitempty"`
}

// GetExecutionTime returns the execution time of the policy
func (p *DeletingPolicy) GetExecutionTime() (*time.Time, error) {
	lastExecutionTime := p.Status.LastExecutionTime.Time
	if lastExecutionTime.IsZero() {
		creationTime := p.GetCreationTimestamp().Time
		return p.GetNextExecutionTime(creationTime)
	} else {
		return p.GetNextExecutionTime(lastExecutionTime)
	}
}

// GetNextExecutionTime returns the next execution time of the policy
func (p *DeletingPolicy) GetNextExecutionTime(time time.Time) (*time.Time, error) {
	cronExpr, err := cronexpr.Parse(p.Spec.Schedule)
	if err != nil {
		return nil, err
	}
	nextExecutionTime := cronExpr.Next(time)
	return &nextExecutionTime, nil
}

func (p *DeletingPolicy) GetKind() string {
	return "DeletingPolicy"
}
