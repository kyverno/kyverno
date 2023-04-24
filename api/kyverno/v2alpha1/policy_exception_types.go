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
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	errs = append(errs, ValidateVariables(p)...)
	errs = append(errs, p.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

func ValidateVariables(p *PolicyException) (errs field.ErrorList) {
	vars := regex.GetVariables(p)
	background := p.Spec.BackgroundProcessingEnabled()
	root := field.NewPath("spec")
	if background {
		if err := p.hasUserVars(vars, root); err != nil {
			errs = append(errs, err...)
		}
	}
	if err := p.hasInvalidVars(background, root); err != nil {
		errs = append(errs, field.Forbidden(root, fmt.Sprintf("policy contains invalid variables: %s", err)))
	}
	return errs
}

func (p *PolicyException) hasInvalidVars(bg bool, path *field.Path) (errs field.ErrorList) {
	if len(regex.GetVariables(p.Spec.Match)) > 0 {
		errs = append(errs, field.Forbidden(path.Child("match"), fmt.Sprintf("policy exception \"%s\" should not have variables in match section", p.Name)))
	}
	return errs
}

func (p *PolicyException) hasUserVars(vars [][]string, path *field.Path) (errs field.ErrorList) {
	if err := hasUserInMatch(p, path); err != nil {
		errs = append(errs, err...)
	}
	if err := regex.HasForbiddenVars(vars); err != nil {
		errs = append(errs, field.Forbidden(path, fmt.Sprintf("%s", err)))
	}
	return errs
}

func hasUserInMatch(p *PolicyException, path *field.Path) (errs field.ErrorList) {
	nxtpth := path.Child("match")
	if len(p.Spec.Match.All) > 0 {
		for i, a := range p.Spec.Match.All {
			if pth := userInfoDefined(a.UserInfo); pth != "" {
				errs = append(errs, field.Forbidden(nxtpth.Child("all").Index(i), "invalid variable used, only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy exception"))
			}
		}
		return errs
	}
	if len(p.Spec.Match.Any) > 0 {
		for i, a := range p.Spec.Match.Any {
			if pth := userInfoDefined(a.UserInfo); pth != "" {
				// return fmt.Errorf("invalid variable used at path: spec/match/any/%d", i)
				errs = append(errs, field.Forbidden(nxtpth.Child("any").Index(i), "invalid variable used, only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy exception"))
			}
		}
		return errs
	}
	return nil
}

func userInfoDefined(ui kyvernov1.UserInfo) string {
	if len(ui.Roles) > 0 {
		return "roles"
	}
	if len(ui.ClusterRoles) > 0 {
		return "clusterRoles"
	}
	if len(ui.Subjects) > 0 {
		return "subjects"
	}
	return ""
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

	// Match defines match clause used to check
	Match kyvernov2beta1.MatchResources `json:"match"`

	// Exceptions is a list policy/rules to be excluded
	Exceptions []Exception `json:"exceptions"`

	// Conditions are used to determine if a resource applies to the exception by evaluating a
	// set of conditions. The declaration can contain nested `any` or `all` statements.
	// +optional
	RawAnyAllConditions *apiextv1.JSON `json:"preconditions,omitempty" yaml:"preconditions,omitempty"`
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

func (p *PolicyExceptionSpec) GetAnyAllConditions() apiextensions.JSON {
	return kyvernov1.FromJSON(p.RawAnyAllConditions)
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}
