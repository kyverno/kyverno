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
package v2beta1

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=polex,categories=kyverno
// +kubebuilder:storageversion

// PolicyException declares resources to be excluded from specified policies.
type PolicyException struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec PolicyExceptionSpec `json:"spec" yaml:"spec"`
}

// Validate implements programmatic validation
func (p *PolicyException) Validate() (errs field.ErrorList) {
	if err := ValidateVariables(p); err != nil {
		errs = append(errs, field.Forbidden(field.NewPath(""), fmt.Sprintf("Policy Exception \"%s\" should not have variables", p.Name)))
	}
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

func ValidateVariables(polex *PolicyException) error {
	return regex.ObjectHasVariables(polex)
}

// Contains returns true if it contains an exception for the given policy/rule pair
func (p *PolicyException) Contains(policy string, rule string) bool {
	return p.Spec.Contains(policy, rule)
}

// PolicyExceptionSpec stores policy exception spec
type PolicyExceptionSpec struct {
	// Background controls if exceptions are applied to existing policies during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	Background *bool `json:"background,omitempty" yaml:"background,omitempty"`

	// Match defines match clause used to check if a resource applies to the exception
	Match MatchResources `json:"match" yaml:"match"`

	// Exceptions is a list policy/rules to be excluded
	Exceptions []Exception `json:"exceptions" yaml:"exceptions"`
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

// Exception stores infos about a policy and rules
type Exception struct {
	// PolicyName identifies the policy to which the exception is applied.
	// The policy name uses the format <namespace>/<name> unless it
	// references a ClusterPolicy.
	PolicyName string `json:"policyName" yaml:"policyName"`

	// RuleNames identifies the rules to which the exception is applied.
	RuleNames []string `json:"ruleNames" yaml:"ruleNames"`
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
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []PolicyException `json:"items" yaml:"items"`
}
