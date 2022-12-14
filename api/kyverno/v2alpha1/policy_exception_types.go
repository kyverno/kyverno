/*
Copyright 2022 The Kubernetes authors.

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
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:shortName=polex,categories=kyverno

// PolicyException declares resources to be excluded from specified policies.
type PolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec PolicyExceptionSpec `json:"spec"`
}

// Validate implements programmatic validation
func (p *PolicyException) Validate() (errs field.ErrorList) {
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	// errs = append(errs, ValidateSchedule(path.Child("schedule"), p.Schedule)...)
	// errs = append(errs, p.MatchResources.Validate(path.Child("match"), namespaced, clusterResources)...)
	// if p.ExcludeResources != nil {
	// 	errs = append(errs, p.ExcludeResources.Validate(path.Child("exclude"), namespaced, clusterResources)...)
	// }
	// errs = append(errs, p.ValidateMatchExcludeConflict(path)...)
	return errs
}

// PolicyExceptionSpec stores policy exception spec
type PolicyExceptionSpec struct {
	// Match defines match clause used to check if a resource applies to the exception
	Match kyvernov2beta1.MatchResources `json:"match"`

	// Exceptions is a list policy/rules to be excluded
	Exceptions []Exception `json:"exceptions"`
}

// Validate implements programmatic validation
func (p *PolicyExceptionSpec) Validate(path *field.Path) (errs field.ErrorList) {
	errs = append(errs, p.Match.Validate(path.Child("match"), false, nil)...)
	exceptionsPath := path.Child("exceptions")
	for i, e := range p.Exceptions {
		errs = append(errs, e.Validate(exceptionsPath.Index(i))...)
	}
	return errs
}

// Exception stores infos about a policy and rules
type Exception struct {
	// PolicyName identifies the policy to which the exception is applied.
	PolicyName string `json:"policyName"`

	// RuleNames identifies the rules to which the exception is applied.
	RuleNames []string `json:"ruleNames"`
}

// Validate implements programmatic validation
func (p *Exception) Validate(path *field.Path) (errs field.ErrorList) {
	if p.PolicyName == "" {
		errs = append(errs, field.Required(path.Child("policyName"), "An exception requires a policy name"))
	}
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
