package v1

import (
	"encoding/json"
	"reflect"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
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

// HasMutateOrValidateOrGenerate checks for rule types
func (p *ClusterPolicy) HasMutateOrValidateOrGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate rule types
func (p *ClusterPolicy) HasMutate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasMutate() {
			return true
		}
	}

	return false
}

// HasValidate checks for validate rule types
func (p *ClusterPolicy) HasValidate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasValidate() {
			return true
		}
	}

	return false
}

// HasGenerate checks for generate rule types
func (p *ClusterPolicy) HasGenerate() bool {
	for _, rule := range p.Spec.Rules {
		if rule.HasGenerate() {
			return true
		}
	}

	return false
}

// HasVerifyImages checks for image verification rule types
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

// MatchKinds returns a slice of all kinds to match
func (r Rule) MatchKinds() []string {
	matchKinds := r.MatchResources.ResourceDescription.Kinds
	for _, value := range r.MatchResources.All {
		matchKinds = append(matchKinds, value.ResourceDescription.Kinds...)
	}
	for _, value := range r.MatchResources.Any {
		matchKinds = append(matchKinds, value.ResourceDescription.Kinds...)
	}

	return matchKinds
}

// ExcludeKinds returns a slice of all kinds to exclude
func (r Rule) ExcludeKinds() []string {
	excludeKinds := r.ExcludeResources.ResourceDescription.Kinds
	for _, value := range r.ExcludeResources.All {
		excludeKinds = append(excludeKinds, value.ResourceDescription.Kinds...)
	}
	for _, value := range r.ExcludeResources.Any {
		excludeKinds = append(excludeKinds, value.ResourceDescription.Kinds...)
	}
	return excludeKinds
}

// DeserializeAnyPattern deserialize apiextensions.JSON to []interface{}
func (in *Validation) DeserializeAnyPattern() ([]interface{}, error) {
	if in.AnyPattern == nil {
		return nil, nil
	}
	res, nil := deserializePattern(in.AnyPattern)
	return res, nil
}

func deserializePattern(pattern apiextensions.JSON) ([]interface{}, error) {
	anyPattern, err := json.Marshal(pattern)
	if err != nil {
		return nil, err
	}

	var res []interface{}
	if err := json.Unmarshal(anyPattern, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func jsonDeepCopy(in apiextensions.JSON) *apiextensions.JSON {
	if in == nil {
		return nil
	}

	out := new(apiextensions.JSON)
	switch in := in.(type) {
	case bool, int64, float64, string:
		*out = in
	case []interface{}:
		out_tmp := make([]interface{}, len(in))

		for i, v := range in {
			if v != nil {
				out_tmp[i] = *jsonDeepCopy(v)
			} else {
				out_tmp[i] = nil
			}
		}

		*out = out_tmp
	case map[string]interface{}:
		out_tmp := make(map[string]interface{})

		for k, v := range in {
			if v != nil {
				out_tmp[k] = *jsonDeepCopy(v)
			} else {
				out_tmp[k] = nil
			}
		}

		*out = out_tmp
	}

	return out
}

// DeepCopyInto is declared because k8s:deepcopy-gen is
// not able to generate this method for interface{} member
func (in *Mutation) DeepCopyInto(out *Mutation) {
	if out == nil {
		return
	}

	*out = *in
	if in.Overlay != nil {
		out.Overlay = *jsonDeepCopy(in.Overlay)
	}

	if in.Patches != nil {
		out.Patches = make([]Patch, len(in.Patches))
		for i, v := range in.Patches {
			v.DeepCopyInto(&out.Patches[i])
		}
	}

	if out.PatchStrategicMerge != nil {
		out.PatchStrategicMerge = *jsonDeepCopy(in.PatchStrategicMerge)
	}

	if in.ForEachMutation != nil {
		out.ForEachMutation = make([]*ForEachMutation, len(in.ForEachMutation))
		for i, v := range in.ForEachMutation {
			out.ForEachMutation[i] = v.DeepCopy()
		}
	}
}

// TODO - the DeepCopyInto methods are added here to work-around
// codegen issues with handling DeepCopy of the apiextensions.JSON
// type. We need to update to apiextensions/v1.JSON which works
// with DeepCopy and remove these methods, or re-write them to
// actually perform a deep copy.
// Also see: https://github.com/kyverno/kyverno/pull/2000

func (pp *Patch) DeepCopyInto(out *Patch) {
	if out == nil {
		return
	}

	*out = *pp
	if pp.Value != nil {
		out.Value = *jsonDeepCopy(pp.Value)
	}
}
func (in *Validation) DeepCopyInto(out *Validation) {
	if out == nil {
		return
	}

	*out = *in
	if in.ForEachValidation != nil {
		out.ForEachValidation = make([]*ForEachValidation, len(in.ForEachValidation))
		for i, v := range in.ForEachValidation {
			out.ForEachValidation[i] = v.DeepCopy()
		}
	}

	if in.Pattern != nil {
		out.Pattern = *jsonDeepCopy(in.Pattern)
	}

	if in.AnyPattern != nil {
		out.AnyPattern = *jsonDeepCopy(in.AnyPattern)
	}

	if in.Deny != nil {
		out.Deny = in.Deny.DeepCopy()
	}
}
func (in *ForEachValidation) DeepCopyInto(out *ForEachValidation) {
	if out == nil {
		return
	}

	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))

		for i, v := range *in {
			(*out)[i] = *v.DeepCopy()
		}
	}

	if in.AnyAllConditions != nil {
		out.AnyAllConditions = in.AnyAllConditions.DeepCopy()
	}

	if in.Pattern != nil {
		out.Pattern = *jsonDeepCopy(in.Pattern)
	}

	if in.AnyPattern != nil {
		out.AnyPattern = *jsonDeepCopy(in.AnyPattern)
	}

	if in.Deny != nil {
		out.Deny = in.Deny.DeepCopy()
	}
}

func (in *ForEachMutation) DeepCopyInto(out *ForEachMutation) {
	if out == nil {
		return
	}

	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))

		for i, v := range *in {
			(*out)[i] = *v.DeepCopy()
		}
	}

	if in.AnyAllConditions != nil {
		out.AnyAllConditions = in.AnyAllConditions.DeepCopy()
	}

	if in.PatchStrategicMerge != nil {
		out.PatchStrategicMerge = *jsonDeepCopy(in.PatchStrategicMerge)
	}
}
func (gen *Generation) DeepCopyInto(out *Generation) {
	if out == nil {
		return
	}

	*out = *gen
	out.ResourceSpec = *gen.ResourceSpec.DeepCopy()

	if out.Data != nil {
		out.Data = *jsonDeepCopy(gen.Data)
	}
}
func (cond *Condition) DeepCopyInto(out *Condition) {
	if out == nil {
		return
	}

	*out = *cond
	if cond.Key != nil {
		out.Key = *jsonDeepCopy(cond.Key)
	}

	if cond.Value != nil {
		out.Value = *jsonDeepCopy(cond.Value)
	}
}
func (in *Deny) DeepCopyInto(out *Deny) {
	if out == nil {
		return
	}

	*out = *in
	if in.AnyAllConditions != nil {
		out.AnyAllConditions = *jsonDeepCopy(in.AnyAllConditions)
	}
}
func (in *Rule) DeepCopyInto(out *Rule) {
	//deepcopy.Copy(in, out)
	//*out = *in

	temp, err := json.Marshal(in)
	if err != nil {
		// never should get here
		return
	}

	err = json.Unmarshal(temp, out)
	if err != nil {
		// never should get here
		return
	}
	// *out = *in
	// if in.Context != nil {
	// 	in, out := &in.Context, &out.Context
	// 	*out = make([]ContextEntry, len(*in))
	// 	for i := range *in {
	// 		(*in)[i].DeepCopyInto(&(*out)[i])
	// 	}
	// }
	// in.MatchResources.DeepCopyInto(&out.MatchResources)
	// in.ExcludeResources.DeepCopyInto(&out.ExcludeResources)
	// if in.AnyAllConditions != nil {
	// 	out.AnyAllConditions = in.AnyAllConditions
	// }
	// in.Mutation.DeepCopyInto(&out.Mutation)
	// in.Validation.DeepCopyInto(&out.Validation)
	// in.Generation.DeepCopyInto(&out.Generation)
	// if in.VerifyImages != nil {
	// 	in, out := &in.VerifyImages, &out.VerifyImages
	// 	*out = make([]*ImageVerification, len(*in))
	// 	for i := range *in {
	// 		if (*in)[i] != nil {
	// 			in, out := &(*in)[i], &(*out)[i]
	// 			*out = new(ImageVerification)
	// 			**out = **in
	// 		}
	// 	}
	// }
}

// ToKey generates the key string used for adding label to polivy violation
func (rs ResourceSpec) ToKey() string {
	return rs.Kind + "." + rs.Name
}

// ViolatedRule stores the information regarding the rule.
type ViolatedRule struct {
	// Name specifies violated rule name.
	Name string `json:"name" yaml:"name"`

	// Type specifies violated rule type.
	Type string `json:"type" yaml:"type"`

	// Message specifies violation message.
	// +optional
	Message string `json:"message" yaml:"message"`

	// Status shows the rule response status
	Status string `json:"status" yaml:"status"`
}
