package v1

import (
	"encoding/json"
	"reflect"
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

func FromJSON(in *apiextv1.JSON) apiextensions.JSON {
	var out apiextensions.JSON
	if err := apiextv1.Convert_v1_JSON_To_apiextensions_JSON(in, &out, nil); err != nil {
		log.Log.Error(err, "failed to convert JSON to interface")
	}
	return out
}

func ToJSON(in apiextensions.JSON) *apiextv1.JSON {
	if in == nil {
		return nil
	}
	var out apiextv1.JSON
	if err := apiextv1.Convert_apiextensions_JSON_To_v1_JSON(&in, &out, nil); err != nil {
		log.Log.Error(err, "failed to convert interface to JSON")
	}
	return &out
}

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
	for _, rule := range p.Spec.GetRules() {
		if rule.HasMutate() || rule.HasValidate() || rule.HasGenerate() {
			return true
		}
	}
	return false
}

// HasMutate checks for mutate rule types
func (p *ClusterPolicy) HasMutate() bool {
	for _, rule := range p.Spec.GetRules() {
		if rule.HasMutate() {
			return true
		}
	}

	return false
}

// HasValidate checks for validate rule types
func (p *ClusterPolicy) HasValidate() bool {
	for _, rule := range p.Spec.GetRules() {
		if rule.HasValidate() {
			return true
		}
	}

	return false
}

// HasGenerate checks for generate rule types
func (p *ClusterPolicy) HasGenerate() bool {
	for _, rule := range p.Spec.GetRules() {
		if rule.HasGenerate() {
			return true
		}
	}

	return false
}

// HasVerifyImages checks for image verification rule types
func (p *ClusterPolicy) HasVerifyImages() bool {
	for _, rule := range p.Spec.GetRules() {
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
	anyPattern := in.GetAnyPattern()
	if anyPattern == nil {
		return nil, nil
	}
	res, nil := deserializePattern(anyPattern)
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
