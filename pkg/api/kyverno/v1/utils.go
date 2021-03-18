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

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (cond *Condition) DeepCopyInto(out *Condition) {
	if out != nil {
		*out = *cond
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
