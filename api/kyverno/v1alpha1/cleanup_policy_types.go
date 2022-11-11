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

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/policy/generate"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CleanupPolicy defines a rule for resource cleanup.
type CleanupPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy behaviors.
	Spec CleanupPolicySpec `json:"spec"`

	// Status contains policy runtime data.
	// +optional
	Status CleanupPolicyStatus `json:"status,omitempty"`
}

// Validate implements programmatic validation
func (p *CleanupPolicy) Validate(clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, kyvernov1.ValidatePolicyName(field.NewPath("metadata").Child("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"), clusterResources, true)...)
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CleanupPolicyList is a list of ClusterPolicy instances.
type CleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CleanupPolicy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterCleanupPolicy defines rule for resource cleanup.
type ClusterCleanupPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy behaviors.
	Spec CleanupPolicySpec `json:"spec"`

	// Status contains policy runtime data.
	// +optional
	Status CleanupPolicyStatus `json:"status,omitempty"`
}

// Validate implements programmatic validation
func (p *ClusterCleanupPolicy) Validate(clusterResources sets.String) (errs field.ErrorList) {
	errs = append(errs, kyvernov1.ValidatePolicyName(field.NewPath("metadata").Child("name"), p.Name)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"), clusterResources, false)...)
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCleanupPolicyList is a list of ClusterCleanupPolicy instances.
type ClusterCleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterCleanupPolicy `json:"items"`
}

// CleanupPolicySpec stores specifications for selecting resources that the user needs to delete
// and schedule when the matching resources needs deleted.
type CleanupPolicySpec struct {
	// MatchResources defines when cleanuppolicy should be applied. The match
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the user name or role.
	// At least one kind is required.
	MatchResources kyvernov1.MatchResources `json:"match,omitempty"`

	// ExcludeResources defines when cleanuppolicy should not be applied. The exclude
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the name or role.
	// +optional
	ExcludeResources kyvernov1.MatchResources `json:"exclude,omitempty"`

	// The schedule in Cron format
	Schedule string `json:"schedule"`

	// Conditions defines conditions used to select resources which user needs to delete
	// +optional
	Conditions *kyvernov1.AnyAllConditions `json:"conditions,omitempty"`
}

// CleanupPolicyStatus stores the status of the policy.
type CleanupPolicyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// Validate implements programmatic validation
func (p *CleanupPolicySpec) Validate(path *field.Path, clusterResources sets.String, namespaced bool) (errs field.ErrorList) {
	errs = append(errs, ValidateSchedule(path.Child("schedule"), p.Schedule)...)
	errs = append(errs, p.MatchResources.Validate(path.Child("match"), namespaced, clusterResources)...)
	errs = append(errs, p.ExcludeResources.Validate(path.Child("exclude"), namespaced, clusterResources)...)
	errs = append(errs, p.ValidateMatchExcludeConflict(path)...)
	return errs
}

// ValidateSchedule validates whether the schedule specified is in proper cron format or not.
func ValidateSchedule(path *field.Path, schedule string) (errs field.ErrorList) {
	if _, err := cron.ParseStandard(schedule); err != nil {
		errs = append(errs, field.Invalid(path, schedule, "schedule spec in the cleanupPolicy is not in proper cron format"))
	}
	return errs
}

// ValidateMatchExcludeConflict checks if the resultant of match and exclude block is not an empty set
func (spec *CleanupPolicySpec) ValidateMatchExcludeConflict(path *field.Path) (errs field.ErrorList) {
	if len(spec.ExcludeResources.All) > 0 || len(spec.MatchResources.All) > 0 {
		return errs
	}
	// if both have any then no resource should be common
	if len(spec.MatchResources.Any) > 0 && len(spec.ExcludeResources.Any) > 0 {
		for _, rmr := range spec.MatchResources.Any {
			for _, rer := range spec.ExcludeResources.Any {
				if reflect.DeepEqual(rmr, rer) {
					return append(errs, field.Invalid(path, spec, "CleanupPolicy is matching an empty set"))
				}
			}
		}
		return errs
	}
	if reflect.DeepEqual(spec.ExcludeResources, kyvernov1.MatchResources{}) {
		return errs
	}
	excludeRoles := sets.NewString(spec.ExcludeResources.Roles...)
	excludeClusterRoles := sets.NewString(spec.ExcludeResources.ClusterRoles...)
	excludeKinds := sets.NewString(spec.ExcludeResources.Kinds...)
	excludeNamespaces := sets.NewString(spec.ExcludeResources.Namespaces...)
	excludeSubjects := sets.NewString()
	for _, subject := range spec.ExcludeResources.Subjects {
		subjectRaw, _ := json.Marshal(subject)
		excludeSubjects.Insert(string(subjectRaw))
	}
	excludeSelectorMatchExpressions := sets.NewString()
	if spec.ExcludeResources.Selector != nil {
		for _, matchExpression := range spec.ExcludeResources.Selector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	excludeNamespaceSelectorMatchExpressions := sets.NewString()
	if spec.ExcludeResources.NamespaceSelector != nil {
		for _, matchExpression := range spec.ExcludeResources.NamespaceSelector.MatchExpressions {
			matchExpressionRaw, _ := json.Marshal(matchExpression)
			excludeNamespaceSelectorMatchExpressions.Insert(string(matchExpressionRaw))
		}
	}
	if len(excludeRoles) > 0 {
		if len(spec.MatchResources.Roles) == 0 || !excludeRoles.HasAll(spec.MatchResources.Roles...) {
			return errs
		}
	}
	if len(excludeClusterRoles) > 0 {
		if len(spec.MatchResources.ClusterRoles) == 0 || !excludeClusterRoles.HasAll(spec.MatchResources.ClusterRoles...) {
			return errs
		}
	}
	if len(excludeSubjects) > 0 {
		if len(spec.MatchResources.Subjects) == 0 {
			return errs
		}
		for _, subject := range spec.MatchResources.UserInfo.Subjects {
			subjectRaw, _ := json.Marshal(subject)
			if !excludeSubjects.Has(string(subjectRaw)) {
				return errs
			}
		}
	}
	if spec.ExcludeResources.Name != "" {
		if !wildcard.Match(spec.ExcludeResources.Name, spec.MatchResources.Name) {
			return errs
		}
	}
	if len(spec.ExcludeResources.Names) > 0 {
		excludeSlice := spec.ExcludeResources.Names
		matchSlice := spec.MatchResources.Names

		// if exclude block has something and match doesn't it means we
		// have a non empty set
		if len(spec.MatchResources.Names) == 0 {
			return errs
		}

		// if *any* name in match and exclude conflicts
		// we want user to fix that
		for _, matchName := range matchSlice {
			for _, excludeName := range excludeSlice {
				if wildcard.Match(excludeName, matchName) {
					return append(errs, field.Invalid(path, spec, "CleanupPolicy is matching an empty set"))
				}
			}
		}
		return errs
	}
	if len(excludeNamespaces) > 0 {
		if len(spec.MatchResources.Namespaces) == 0 || !excludeNamespaces.HasAll(spec.MatchResources.Namespaces...) {
			return errs
		}
	}
	if len(excludeKinds) > 0 {
		if len(spec.MatchResources.Kinds) == 0 || !excludeKinds.HasAll(spec.MatchResources.Kinds...) {
			return errs
		}
	}
	if spec.MatchResources.Selector != nil && spec.ExcludeResources.Selector != nil {
		if len(excludeSelectorMatchExpressions) > 0 {
			if len(spec.MatchResources.Selector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range spec.MatchResources.Selector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(spec.ExcludeResources.Selector.MatchLabels) > 0 {
			if len(spec.MatchResources.Selector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range spec.MatchResources.Selector.MatchLabels {
				if spec.ExcludeResources.Selector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if spec.MatchResources.NamespaceSelector != nil && spec.ExcludeResources.NamespaceSelector != nil {
		if len(excludeNamespaceSelectorMatchExpressions) > 0 {
			if len(spec.MatchResources.NamespaceSelector.MatchExpressions) == 0 {
				return errs
			}
			for _, matchExpression := range spec.MatchResources.NamespaceSelector.MatchExpressions {
				matchExpressionRaw, _ := json.Marshal(matchExpression)
				if !excludeNamespaceSelectorMatchExpressions.Has(string(matchExpressionRaw)) {
					return errs
				}
			}
		}
		if len(spec.ExcludeResources.NamespaceSelector.MatchLabels) > 0 {
			if len(spec.MatchResources.NamespaceSelector.MatchLabels) == 0 {
				return errs
			}
			for label, value := range spec.MatchResources.NamespaceSelector.MatchLabels {
				if spec.ExcludeResources.NamespaceSelector.MatchLabels[label] != value {
					return errs
				}
			}
		}
	}
	if (spec.MatchResources.Selector == nil && spec.ExcludeResources.Selector != nil) ||
		(spec.MatchResources.Selector != nil && spec.ExcludeResources.Selector == nil) {
		return errs
	}
	if (spec.MatchResources.NamespaceSelector == nil && spec.ExcludeResources.NamespaceSelector != nil) ||
		(spec.MatchResources.NamespaceSelector != nil && spec.ExcludeResources.NamespaceSelector == nil) {
		return errs
	}
	if spec.MatchResources.Annotations != nil && spec.ExcludeResources.Annotations != nil {
		if !(reflect.DeepEqual(spec.MatchResources.Annotations, spec.ExcludeResources.Annotations)) {
			return errs
		}
	}
	if (spec.MatchResources.Annotations == nil && spec.ExcludeResources.Annotations != nil) ||
		(spec.MatchResources.Annotations != nil && spec.ExcludeResources.Annotations == nil) {
		return errs
	}
	return append(errs, field.Invalid(path, spec, "CleanupPolicy is matching an empty set"))
}

// Cleanup provides implementation to validate permission for using DELETE operation by CleanupPolicy
type Cleanup struct {
	// rule to hold CleanupPolicy specifications
	spec CleanupPolicySpec
	// authCheck to check access for operations
	authCheck generate.Operations
	// logger
	log logr.Logger
}

// NewCleanup returns a new instance of Cleanup validation checker
func NewCleanup(client dclient.Interface, cleanup CleanupPolicySpec, log logr.Logger) *Cleanup {
	c := Cleanup{
		spec:      cleanup,
		authCheck: generate.NewAuth(client, log),
		log:       log,
	}

	return &c
}

// canIDelete returns a error if kyverno cannot perform operations
func (c *Cleanup) CanIDelete(kind, namespace string) error {
	// Skip if there is variable defined
	authCheck := c.authCheck
	if !variables.IsVariable(kind) && !variables.IsVariable(namespace) {
		// DELETE
		ok, err := authCheck.CanIDelete(kind, namespace)
		if err != nil {
			// machinery error
			return err
		}
		if !ok {
			return fmt.Errorf("kyverno does not have permissions to 'delete' resource %s/%s. Update permissions in ClusterRole", kind, namespace)
		}
	} else {
		c.log.V(4).Info("name & namespace uses variables, so cannot be resolved. Skipping Auth Checks.")
	}

	return nil
}
