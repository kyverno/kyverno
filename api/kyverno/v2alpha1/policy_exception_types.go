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

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=polex,categories=kyverno
// +kubebuilder:deprecatedversion

// PolicyException declares resources to be excluded from specified policies.
type PolicyException kyvernov2beta1.PolicyException

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

// Exception stores infos about a policy and rules
type Exception = kyvernov2beta1.Exception

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList kyvernov2beta1.PolicyExceptionList
