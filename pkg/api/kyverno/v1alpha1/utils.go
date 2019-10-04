package v1alpha1

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func (p ClusterPolicy) HasMutateOrValidate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() {
			return true
		}
	}
	return false
}
func (p ClusterPolicy) HasMutate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() {
			return true
		}
	}
	return false
}

func (p ClusterPolicy) HasValidate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasValidate() {
			return true
		}
	}
	return false
}

func (p ClusterPolicy) HasGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasGenerate() {
			return true
		}
	}
	return false
}

func (r Rule) HasMutate() bool {
	return !reflect.DeepEqual(r.Mutation, Mutation{})
}

func (r Rule) HasValidate() bool {
	return !reflect.DeepEqual(r.Validation, Validation{})
}

func (r Rule) HasGenerate() bool {
	return !reflect.DeepEqual(r.Generation, Generation{})
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Mutation) DeepCopyInto(out *Mutation) {
	if out != nil {
		*out = *in
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (pp *Patch) DeepCopyInto(out *Patch) {
	if out != nil {
		*out = *pp
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Validation) DeepCopyInto(out *Validation) {
	if out != nil {
		*out = *in
	}
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (gen *Generation) DeepCopyInto(out *Generation) {
	if out != nil {
		*out = *gen
	}
}

//ToKey generates the key string used for adding label to polivy violation
func (rs ResourceSpec) ToKey() string {
	if rs.Namespace == "" {
		return rs.Kind + "." + rs.Name
	}
	return rs.Kind + "." + rs.Namespace + "." + rs.Name
}

// joinErrs joins the list of error into single error
// adds a new line between errors
func joinErrs(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	res := "\n"
	for _, err := range errs {
		res = fmt.Sprintf(res + err.Error() + "\n")
	}

	return errors.New(res)
}

//Contains Check if strint is contained in a list of string
func containString(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}

// hasExistingAnchor checks if str has existing anchor
// strip anchor if necessary
func hasExistingAnchor(str string) (bool, string) {
	left := "^("
	right := ")"

	if len(str) < len(left)+len(right) {
		return false, str
	}

	return (str[:len(left)] == left && str[len(str)-len(right):] == right), str[len(left) : len(str)-len(right)]
}

// hasValidAnchors checks str has the valid anchor
// mutate: (), +()
// validate: (), ^(), =()
// generate: none
// invalid anchors: ~(),!()
func hasValidAnchors(anchors []anchor, str string) (bool, string) {
	if wrappedWithAttributes(str) {
		return mustWrapWithAnchors(anchors, str)
	}

	return true, str
}

// mustWrapWithAnchors validates str must wrap with
// at least one given anchor
func mustWrapWithAnchors(anchors []anchor, str string) (bool, string) {
	for _, a := range anchors {
		if str[:len(a.left)] == a.left && str[len(str)-len(a.right):] == a.right {
			return true, str[len(a.left) : len(str)-len(a.right)]
		}
	}

	return false, str
}

func wrappedWithAttributes(str string) bool {
	if len(str) < 2 {
		return false
	}

	if (str[0] == '(' && str[len(str)-1] == ')') ||
		(str[0] == '^' || str[0] == '+' || str[0] == '=' || str[0] == '!' || str[0] == '~') &&
			(str[1] == '(' && str[len(str)-1] == ')') {
		return true
	}

	return false
}

func joinAnchors(anchorPatterns []anchor) string {
	var res []string
	for _, a := range anchorPatterns {
		res = append(res, a.left+a.right)
	}

	return strings.Join(res, " || ")
}
