package v1alpha1

import (
	"errors"
	"fmt"
)

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
