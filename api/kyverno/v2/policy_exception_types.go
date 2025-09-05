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
// +kubebuilder:storageversion

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

// Exception stores infos about a policy and rules
type Exception struct {
	// PolicyName identifies the policy to which the exception is applied.
	// The policy name uses the format <namespace>/<name> unless it
	// references a ClusterPolicy.
	PolicyName string `json:"policyName"`

	// RuleNames identifies the rules to which the exception is applied.
	RuleNames []string `json:"ruleNames"`

	// Images specifies image-based exceptions for verifyImages rules.
	// This allows exempting specific images from image verification policies.
	// +optional
	Images []ImageException `json:"images,omitempty"`

	// Values specifies value-based exceptions for validation rules.
	// This allows exempting specific values from validation policies.
	// +optional
	Values []ValueException `json:"values,omitempty"`

	// ReportAs specifies how this exception should be reported.
	// Options: "skip" (default), "warn", "pass"
	// +optional
	// +kubebuilder:default="skip"
	// +kubebuilder:validation:Enum=skip;warn;pass
	ReportAs *ExceptionReportMode `json:"reportAs,omitempty"`
}

// Validate implements programmatic validation
func (p *Exception) Validate(path *field.Path) (errs field.ErrorList) {
	if p.PolicyName == "" {
		errs = append(errs, field.Required(path.Child("policyName"), "An exception requires a policy name"))
	}

	// Validate image exceptions
	imagesPath := path.Child("images")
	for i, imageException := range p.Images {
		errs = append(errs, imageException.Validate(imagesPath.Index(i))...)
	}

	// Validate value exceptions
	valuesPath := path.Child("values")
	for i, valueException := range p.Values {
		errs = append(errs, valueException.Validate(valuesPath.Index(i))...)
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

// IsFinegrained returns true if this exception contains fine-grained controls
func (p *Exception) IsFinegrained() bool {
	return len(p.Images) > 0 || len(p.Values) > 0
}

// HasImageExceptions returns true if this exception contains image-based exceptions
func (p *Exception) HasImageExceptions() bool {
	return len(p.Images) > 0
}

// HasValueExceptions returns true if this exception contains value-based exceptions
func (p *Exception) HasValueExceptions() bool {
	return len(p.Values) > 0
}

// GetReportMode returns the configured reporting mode, defaulting to skip
func (p *Exception) GetReportMode() ExceptionReportMode {
	if p.ReportAs == nil {
		return ExceptionReportSkip
	}
	return *p.ReportAs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PolicyExceptionList is a list of Policy Exceptions
type PolicyExceptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PolicyException `json:"items"`
}

// ExceptionReportMode specifies how exceptions should be reported
// +kubebuilder:validation:Enum=skip;warn;pass
type ExceptionReportMode string

const (
	// ExceptionReportSkip skips the rule entirely (default behavior)
	ExceptionReportSkip ExceptionReportMode = "skip"
	// ExceptionReportWarn generates a warning but allows the resource
	ExceptionReportWarn ExceptionReportMode = "warn"
	// ExceptionReportPass reports the rule as passed
	ExceptionReportPass ExceptionReportMode = "pass"
)

// ImageException specifies image-based exception criteria
type ImageException struct {
	// ImageReferences is a list of image reference patterns to be exempted.
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	ImageReferences []string `json:"imageReferences"`
}

// Validate implements programmatic validation for ImageException
func (i *ImageException) Validate(path *field.Path) (errs field.ErrorList) {
	if len(i.ImageReferences) == 0 {
		errs = append(errs, field.Required(path.Child("imageReferences"), "Image exception requires at least one image reference"))
	}
	return errs
}

// ValueException specifies value-based exception criteria
type ValueException struct {
	// Path is a JSONPath expression that identifies the field to check for exempted values.
	// For example: "spec.containers[*].securityContext.runAsUser" or "metadata.labels.environment"
	Path string `json:"path"`

	// Values is a list of values that should be exempted from validation.
	Values []string `json:"values"`

	// Operator specifies how values should be matched against the exempted values.
	// Supported operators: "equals" (default), "in", "startsWith", "endsWith", "contains"
	// +optional
	// +kubebuilder:default="equals"
	// +kubebuilder:validation:Enum=equals;in;startsWith;endsWith;contains
	Operator *ValueOperator `json:"operator,omitempty"`
}

// Validate implements programmatic validation for ValueException
func (v *ValueException) Validate(path *field.Path) (errs field.ErrorList) {
	if v.Path == "" {
		errs = append(errs, field.Required(path.Child("path"), "Value exception requires a path"))
	}
	if len(v.Values) == 0 {
		errs = append(errs, field.Required(path.Child("values"), "Value exception requires at least one value"))
	}
	return errs
}

// ValueOperator specifies how values should be matched
// +kubebuilder:validation:Enum=equals;in;startsWith;endsWith;contains
type ValueOperator string

const (
	// ValueOperatorEquals matches exact values
	ValueOperatorEquals ValueOperator = "equals"
	// ValueOperatorIn checks if value is in the list
	ValueOperatorIn ValueOperator = "in"
	// ValueOperatorStartsWith checks if value starts with any of the exempted values
	ValueOperatorStartsWith ValueOperator = "startsWith"
	// ValueOperatorEndsWith checks if value ends with any of the exempted values
	ValueOperatorEndsWith ValueOperator = "endsWith"
	// ValueOperatorContains checks if value contains any of the exempted values
	ValueOperatorContains ValueOperator = "contains"
)
