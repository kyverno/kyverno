package v1

import (
	"encoding/json"
	"reflect"
	"strings"
)

// HasAutoGenAnnotation checks if a policy has auto-gen annotation
func (p *ClusterPolicy) HasAutoGenAnnotation() bool {
	annotations := p.GetAnnotations()
	val, ok := annotations["pod-policies.kyverno.io/autogen-controllers"]
	if ok && strings.ToLower(val) != "none" {
		return true
	}

	return false
}

//HasMutateOrValidateOrGenerate checks for rule types
func (p *ClusterPolicy) HasMutateOrValidateOrGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

//HasMutate checks for mutate rule types
func (p *ClusterPolicy) HasMutate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() {
			return true
		}
	}

	return false
}

//HasVerifyImages checks for image verification rule types
func (p *ClusterPolicy) HasVerifyImages() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasVerifyImages() {
			return true
		}
	}

	return false
}

// BackgroundProcessingEnabled checks if background is set to true
func (p *ClusterPolicy) BackgroundProcessingEnabled() bool {
	if p.Spec.Background == nil {
		return true
	}

	return *p.Spec.Background
}

// HasMutate checks for mutate rule
func (r Rule) HasMutate() bool {
	return !reflect.DeepEqual(r.Mutation, Mutation{})
}

// HasVerifyImages checks for verifyImages rule
func (r Rule) HasVerifyImages() bool {
	return r.VerifyImages != nil && !reflect.DeepEqual(r.VerifyImages, ImageVerification{})
}

// HasValidate checks for validate rule
func (r Rule) HasValidate() bool {
	return !reflect.DeepEqual(r.Validation, Validation{})
}

// HasGenerate checks for generate rule
func (r Rule) HasGenerate() bool {
	return !reflect.DeepEqual(r.Generation, Generation{})
}

// DeserializeAnyPattern deserialize apiextensions.JSON to []interface{}
func (in *Validation) DeserializeAnyPattern() ([]interface{}, error) {
	if in.AnyPattern == nil {
		return nil, nil
	}

	anyPattern, err := json.Marshal(in.AnyPattern)
	if err != nil {
		return nil, err
	}

	var res []interface{}
	if err := json.Unmarshal(anyPattern, &res); err != nil {
		return nil, err
	}

	return res, nil
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Mutation) DeepCopyInto(out *Mutation) {
	if out != nil {
		*out = *in
	}
}

// TODO - the DeepCopyInto methods are added here to work-around
// codegen issues with handling DeepCopy of the apiextensions.JSON
// type. We need to update to apiextensions/v1.JSON which works
// with DeepCopy and remove these methods, or re-write them to
// actually perform a deep copy.
// Also see: https://github.com/kyverno/kyverno/pull/2000

func (pp *Patch) DeepCopyInto(out *Patch) {
	if out != nil {
		*out = *pp
	}
}
func (in *Validation) DeepCopyInto(out *Validation) {
	if out != nil {
		*out = *in
	}
}
func (gen *Generation) DeepCopyInto(out *Generation) {
	if out != nil {
		*out = *gen
	}
}
func (cond *Condition) DeepCopyInto(out *Condition) {
	if out != nil {
		*out = *cond
	}
}
func (in *Deny) DeepCopyInto(out *Deny) {
	*out = *in
	if in.AnyAllConditions != nil {
		out.AnyAllConditions = in.AnyAllConditions
	}
}
func (in *Rule) DeepCopyInto(out *Rule) {
	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.MatchResources.DeepCopyInto(&out.MatchResources)
	in.ExcludeResources.DeepCopyInto(&out.ExcludeResources)
	if in.AnyAllConditions != nil {
		out.AnyAllConditions = in.AnyAllConditions
	}
	in.Mutation.DeepCopyInto(&out.Mutation)
	in.Validation.DeepCopyInto(&out.Validation)
	in.Generation.DeepCopyInto(&out.Generation)
	if in.VerifyImages != nil {
		in, out := &in.VerifyImages, &out.VerifyImages
		*out = make([]*ImageVerification, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ImageVerification)
				**out = **in
			}
		}
	}
}

//ToKey generates the key string used for adding label to polivy violation
func (rs ResourceSpec) ToKey() string {
	return rs.Kind + "." + rs.Name
}

// ViolatedRule stores the information regarding the rule.
type ViolatedRule struct {
	// Specifies violated rule name.
	Name string `json:"name" yaml:"name"`

	// Specifies violated rule type.
	Type string `json:"type" yaml:"type"`

	// Specifies violation message.
	// +optional
	Message string `json:"message" yaml:"message"`

	// +optional
	Check string `json:"check" yaml:"check"`
}
