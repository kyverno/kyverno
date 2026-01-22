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
package v2

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/ext/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=polex,categories=kyverno
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="EXPIRED",type=boolean,JSONPath=`.status.expired`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// PolicyException declares resources to be excluded from specified policies.
type PolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec PolicyExceptionSpec `json:"spec"`

	// Status contains policy exception runtime data.
	// +optional
	Status PolicyExceptionStatus `json:"status,omitempty"`
}

// Validate implements programmatic validation
func (p *PolicyException) Validate() (errs field.ErrorList) {
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// Contains returns true if it contains an exception for the given policy/rule pair
func (p *PolicyException) Contains(policy string, rule string) bool {
	return p.Spec.Contains(policy, rule)
}

func (p *PolicyException) GetKind() string {
	return "PolicyException"
}

// HasPodSecurity checks if podSecurity controls is specified
func (p *PolicyException) HasPodSecurity() bool {
	return len(p.Spec.PodSecurity) > 0
}

// PolicyExceptionSpec stores policy exception spec
type PolicyExceptionSpec struct {
	// Background controls if exceptions are applied to existing policies during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	Background *bool `json:"background,omitempty"`

	// Match defines match clause used to check if a resource applies to the exception
	Match kyvernov2beta1.MatchResources `json:"match"`

	// Conditions are used to determine if a resource applies to the exception by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// +optional
	Conditions *AnyAllConditions `json:"conditions,omitempty"`

	// Exceptions is a list policy/rules to be excluded
	Exceptions []Exception `json:"exceptions"`

	// PodSecurity specifies the Pod Security Standard controls to be excluded.
	// Applicable only to policies that have validate.podSecurity subrule.
	// +optional
	PodSecurity []kyvernov1.PodSecurityStandard `json:"podSecurity,omitempty"`

	// ExpiresAt specifies when this exception should stop being applied.
	// The exception will be ignored after this time (not deleted).
	// Format: RFC3339 timestamp (e.g., "2026-12-31T23:59:59Z")
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}

func (p *PolicyExceptionSpec) BackgroundProcessingEnabled() bool {
	if p.Background == nil {
		return true
	}
	return *p.Background
}

// Validate implements programmatic validation
func (p *PolicyExceptionSpec) Validate(path *field.Path) (errs field.ErrorList) {
	if p.BackgroundProcessingEnabled() {
		if userErrs := p.Match.ValidateNoUserInfo(path.Child("match")); len(userErrs) > 0 {
			errs = append(errs, userErrs...)
		}
	}
	errs = append(errs, p.Match.Validate(path.Child("match"), false, nil)...)
	exceptionsPath := path.Child("exceptions")
	for i, e := range p.Exceptions {
		errs = append(errs, e.Validate(exceptionsPath.Index(i))...)
	}

	podSecuityPath := path.Child("podSecurity")
	for i, p := range p.PodSecurity {
		errs = append(errs, p.Validate(podSecuityPath.Index(i))...)
	}
	return errs
}

// Contains returns true if it contains an exception for the given policy/rule pair
func (p *PolicyExceptionSpec) Contains(policy string, rule string) bool {
	for _, exception := range p.Exceptions {
		if exception.Contains(policy, rule) {
			return true
		}
	}
	return false
}

// PolicyExceptionStatus contains runtime data for policy exceptions.
type PolicyExceptionStatus struct {
	// Expired indicates whether the exception has expired based on the expiresAt field.
	// When true, the exception is not applied during policy evaluation.
	// +optional
	Expired bool `json:"expired,omitempty"`

	// Conditions represent the latest observed conditions of the policy exception.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Exception stores infos about a policy and rules
type Exception struct {
	// PolicyName identifies the policy to which the exception is applied.
	// The policy name uses the format <namespace>/<name> unless it
	// references a ClusterPolicy.
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

// Contains returns true if it contains an exception for the given policy/rule pair
func (p *Exception) Contains(policy string, rule string) bool {
	if p.PolicyName == policy {
		for _, ruleName := range p.RuleNames {
			if wildcard.Match(ruleName, rule) {
				return true
			}
		}
	}
	return false
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
