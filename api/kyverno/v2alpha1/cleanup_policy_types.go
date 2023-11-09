/*
Copyright 2020 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2alpha1

import (
	"time"

	"github.com/aptible/supercronic/cronexpr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/robfig/cron"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=cleanpol,categories=kyverno
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion

// CleanupPolicy defines a rule for resource cleanup.
type CleanupPolicy kyvernov2beta1.CleanupPolicy

// GetSpec returns the policy spec
func (p *CleanupPolicy) GetSpec() *CleanupPolicySpec {
	return &p.Spec
}

// GetStatus returns the policy status
func (p *CleanupPolicy) GetStatus() *CleanupPolicyStatus {
	return &p.Status
}

// GetExecutionTime returns the execution time of the policy
func (p *CleanupPolicy) GetExecutionTime() (*time.Time, error) {
	lastExecutionTime := p.Status.LastExecutionTime.Time
	if lastExecutionTime.IsZero() {
		creationTime := p.GetCreationTimestamp().Time
		return p.GetNextExecutionTime(creationTime)
	} else {
		return p.GetNextExecutionTime(lastExecutionTime)
	}
}

// GetNextExecutionTime returns the next execution time of the policy
func (p *CleanupPolicy) GetNextExecutionTime(time time.Time) (*time.Time, error) {
	cronExpr, err := cronexpr.Parse(p.Spec.Schedule)
	if err != nil {
		return nil, err
	}
	nextExecutionTime := cronExpr.Next(time)
	return &nextExecutionTime, nil
}

// Validate implements programmatic validation
func (p *CleanupPolicy) Validate(clusterResources sets.Set[string]) (errs field.ErrorList) {
	errs = append(errs, kyvernov1.ValidatePolicyName(field.NewPath("metadata").Child("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"), clusterResources, true)...)
	return errs
}

// GetKind returns the resource kind
func (p *CleanupPolicy) GetKind() string {
	return "CleanupPolicy"
}

// GetAPIVersion returns the resource kind
func (p *CleanupPolicy) GetAPIVersion() string {
	return p.APIVersion
}

// IsNamespaced indicates if the policy is namespace scoped
func (p *CleanupPolicy) IsNamespaced() bool {
	return true
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CleanupPolicyList is a list of ClusterPolicy instances.
type CleanupPolicyList kyvernov2beta1.CleanupPolicyList

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=ccleanpol,categories=kyverno
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:deprecatedversion

// ClusterCleanupPolicy defines rule for resource cleanup.
type ClusterCleanupPolicy kyvernov2beta1.ClusterCleanupPolicy

// GetSpec returns the policy spec
func (p *ClusterCleanupPolicy) GetSpec() *CleanupPolicySpec {
	return &p.Spec
}

// GetStatus returns the policy status
func (p *ClusterCleanupPolicy) GetStatus() *CleanupPolicyStatus {
	return &p.Status
}

// GetExecutionTime returns the execution time of the policy
func (p *ClusterCleanupPolicy) GetExecutionTime() (*time.Time, error) {
	lastExecutionTime := p.Status.LastExecutionTime.Time
	if lastExecutionTime.IsZero() {
		creationTime := p.GetCreationTimestamp().Time
		return p.GetNextExecutionTime(creationTime)
	} else {
		return p.GetNextExecutionTime(lastExecutionTime)
	}
}

// GetNextExecutionTime returns the next execution time of the policy
func (p *ClusterCleanupPolicy) GetNextExecutionTime(time time.Time) (*time.Time, error) {
	cronExpr, err := cronexpr.Parse(p.Spec.Schedule)
	if err != nil {
		return nil, err
	}
	nextExecutionTime := cronExpr.Next(time)
	return &nextExecutionTime, nil
}

// GetKind returns the resource kind
func (p *ClusterCleanupPolicy) GetKind() string {
	return "ClusterCleanupPolicy"
}

// GetAPIVersion returns the resource kind
func (p *ClusterCleanupPolicy) GetAPIVersion() string {
	return p.APIVersion
}

// IsNamespaced indicates if the policy is namespace scoped
func (p *ClusterCleanupPolicy) IsNamespaced() bool {
	return false
}

// Validate implements programmatic validation
func (p *ClusterCleanupPolicy) Validate(clusterResources sets.Set[string]) (errs field.ErrorList) {
	errs = append(errs, kyvernov1.ValidatePolicyName(field.NewPath("metadata").Child("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"), clusterResources, false)...)
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCleanupPolicyList is a list of ClusterCleanupPolicy instances.
type ClusterCleanupPolicyList kyvernov2beta1.ClusterCleanupPolicyList

// CleanupPolicySpec stores specifications for selecting resources that the user needs to delete
// and schedule when the matching resources needs deleted.
type CleanupPolicySpec = kyvernov2beta1.CleanupPolicySpec

// CleanupPolicyStatus stores the status of the policy.
type CleanupPolicyStatus = kyvernov2beta1.CleanupPolicyStatus

func ValidateContext(path *field.Path, context []kyvernov1.ContextEntry) (errs field.ErrorList) {
	for _, entry := range context {
		if entry.ImageRegistry != nil {
			errs = append(errs, field.Invalid(path, context, "ImageRegistry is not allowed in CleanUp Policy"))
		} else if entry.ConfigMap != nil {
			errs = append(errs, field.Invalid(path, context, "ConfigMap is not allowed in CleanUp Policy"))
		}
	}
	return errs
}

// ValidateSchedule validates whether the schedule specified is in proper cron format or not.
func ValidateSchedule(path *field.Path, schedule string) (errs field.ErrorList) {
	if _, err := cron.ParseStandard(schedule); err != nil {
		errs = append(errs, field.Invalid(path, schedule, "schedule spec in the cleanupPolicy is not in proper cron format"))
	}
	return errs
}
