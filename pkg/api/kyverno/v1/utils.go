package v1

import "reflect"

func (p ClusterPolicy) HasMutateOrValidate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() {
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
	return rs.Kind + "." + rs.Name
}
