package annotations

import (
	"encoding/json"
	"reflect"

	"github.com/golang/glog"
	pinfo "github.com/nirmata/kyverno/pkg/info"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//Policy information for annotations
type Policy struct {
	Status string `json:"status"`
	// Key Type/Name
	MutationRules   map[string]Rule `json:"mutationrules,omitempty"`
	ValidationRules map[string]Rule `json:"validationrules,omitempty"`
	GenerationRules map[string]Rule `json:"generationrules,omitempty"`
}

//Rule information for annotations
type Rule struct {
	Status  string `json:"status"`
	Changes string `json:"changes,omitempty"` // TODO for mutation changes
	Message string `json:"message,omitempty"`
}

func (p *Policy) getOverAllStatus() string {
	// mutation
	for _, v := range p.MutationRules {
		if v.Status == "Failure" {
			return "Failure"
		}
	}
	// validation
	for _, v := range p.ValidationRules {
		if v.Status == "Failure" {
			return "Failure"
		}
	}
	// generation
	for _, v := range p.GenerationRules {
		if v.Status == "Failure" {
			return "Failure"
		}
	}
	return "Success"
}

func getRules(rules []*pinfo.RuleInfo, ruleType pinfo.RuleType) map[string]Rule {
	if len(rules) == 0 {
		return nil
	}
	annrules := make(map[string]Rule, 0)
	// var annrules map[string]Rule
	for _, r := range rules {
		if r.RuleType != ruleType {
			continue
		}

		rule := Rule{Status: getStatus(r.IsSuccessful())}
		if !r.IsSuccessful() {
			rule.Message = r.GetErrorString()
		} else {
			if ruleType == pinfo.Mutation {
				// If ruleType is Mutation
				// then for succesful mutation we store the json patch being applied in the annotation information
				rule.Changes = r.Changes
			}
		}
		annrules[r.Name] = rule
	}
	return annrules
}

func (p *Policy) updatePolicy(obj *Policy, ruleType pinfo.RuleType) bool {
	updates := false
	// Check Mutation rules
	switch ruleType {
	case pinfo.Mutation:
		if p.compareMutationRules(obj.MutationRules) {
			updates = true
		}
	case pinfo.Validation:
		if p.compareValidationRules(obj.ValidationRules) {
			updates = true
		}
	case pinfo.Generation:
		if p.compareGenerationRules(obj.GenerationRules) {
			updates = true
		}
		if p.Status != obj.Status {
			updates = true
		}
	}
	// check if any rules failed
	p.Status = p.getOverAllStatus()
	// If there are any updates then the annotation can be updated, can skip
	return updates
}

func (p *Policy) compareMutationRules(rules map[string]Rule) bool {
	// check if the rules have changed
	if !reflect.DeepEqual(p.MutationRules, rules) {
		p.MutationRules = rules
		return true
	}
	return false
}

func (p *Policy) compareValidationRules(rules map[string]Rule) bool {
	// check if the rules have changed
	if !reflect.DeepEqual(p.ValidationRules, rules) {
		p.ValidationRules = rules
		return true
	}
	return false
}

func (p *Policy) compareGenerationRules(rules map[string]Rule) bool {
	// check if the rules have changed
	if !reflect.DeepEqual(p.GenerationRules, rules) {
		p.GenerationRules = rules
		return true
	}
	return false
}

func newAnnotationForPolicy(pi *pinfo.PolicyInfo) *Policy {
	return &Policy{Status: getStatus(pi.IsSuccessful()),
		MutationRules:   getRules(pi.Rules, pinfo.Mutation),
		ValidationRules: getRules(pi.Rules, pinfo.Validation),
		GenerationRules: getRules(pi.Rules, pinfo.Generation),
	}
}

//AddPolicy will add policy annotation if not present or update if present
// modifies obj
// returns true, if there is any update -> caller need to update the obj
// returns false, if there is no change -> caller can skip the update
func AddPolicy(obj *unstructured.Unstructured, pi *pinfo.PolicyInfo, ruleType pinfo.RuleType) bool {
	PolicyObj := newAnnotationForPolicy(pi)
	// get annotation
	ann := obj.GetAnnotations()
	// check if policy already has annotation
	cPolicy, ok := ann[BuildKey(pi.Name)]
	if !ok {
		PolicyByte, err := json.Marshal(PolicyObj)
		if err != nil {
			glog.Error(err)
			return false
		}
		// insert policy information
		ann[BuildKey(pi.Name)] = string(PolicyByte)
		// set annotation back to unstr
		obj.SetAnnotations(ann)
		return true
	}
	cPolicyObj := Policy{}
	err := json.Unmarshal([]byte(cPolicy), &cPolicyObj)
	if err != nil {
		return false
	}
	// update policy information inside the annotation
	// 1> policy status
	// 2> Mutation, Validation, Generation
	if cPolicyObj.updatePolicy(PolicyObj, ruleType) {

		cPolicyByte, err := json.Marshal(cPolicyObj)
		if err != nil {
			return false
		}
		// update policy information
		ann[BuildKey(pi.Name)] = string(cPolicyByte)
		// set annotation back to unstr
		obj.SetAnnotations(ann)
		return true
	}
	return false
}

//RemovePolicy to remove annotations
// return true -> if there was an entry and we deleted it
// return false -> if there is no entry, caller need not update
func RemovePolicy(obj *unstructured.Unstructured, policy string) bool {
	// get annotations
	ann := obj.GetAnnotations()
	if ann == nil {
		return false
	}
	if _, ok := ann[BuildKey(policy)]; !ok {
		return false
	}
	delete(ann, BuildKey(policy))
	// set annotation back to unstr
	obj.SetAnnotations(ann)
	return true
}

//ParseAnnotationsFromObject extracts annotations from the JSON obj
func ParseAnnotationsFromObject(bytes []byte) map[string]string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)
	meta, ok := objectJSON["metadata"].(map[string]interface{})
	if !ok {
		glog.Error("unable to parse")
		return nil
	}
	ann, ok, err := unstructured.NestedStringMap(meta, "annotations")
	if err != nil || !ok {
		return nil
	}
	return ann
}

//AddPolicyJSONPatch generate JSON Patch to add policy informatino JSON patch
func AddPolicyJSONPatch(ann map[string]string, pi *pinfo.PolicyInfo, ruleType pinfo.RuleType) (map[string]string, []byte, error) {
	if !pi.ContainsRuleType(ruleType) {
		return nil, nil, nil
	}
	PolicyObj := newAnnotationForPolicy(pi)
	if ann == nil {
		ann = make(map[string]string, 0)
		PolicyByte, err := json.Marshal(PolicyObj)
		if err != nil {
			return nil, nil, err
		}
		// create a json patch to add annotation object
		ann[BuildKey(pi.Name)] = string(PolicyByte)
		// create add JSON patch
		jsonPatch, err := createAddJSONPatch(BuildKey(pi.Name), string(PolicyByte))
		return ann, jsonPatch, err
	}
	// if the annotations map is present then we
	cPolicy, ok := ann[BuildKey(pi.Name)]
	if !ok {
		PolicyByte, err := json.Marshal(PolicyObj)
		if err != nil {
			return nil, nil, err
		}
		// insert policy information
		ann[BuildKey(pi.Name)] = string(PolicyByte)
		// create add JSON patch
		jsonPatch, err := createAddJSONPatch(BuildKey(pi.Name), string(PolicyByte))

		return ann, jsonPatch, err
	}
	cPolicyObj := Policy{}
	err := json.Unmarshal([]byte(cPolicy), &cPolicyObj)
	// update policy information inside the annotation
	// 1> policy status
	// 2> rule (name, status,changes,type)
	update := cPolicyObj.updatePolicy(PolicyObj, ruleType)
	if !update {
		return nil, nil, err
	}

	cPolicyByte, err := json.Marshal(cPolicyObj)
	if err != nil {
		return nil, nil, err
	}
	// update policy information
	ann[BuildKey(pi.Name)] = string(cPolicyByte)
	// create update JSON patch
	jsonPatch, err := createReplaceJSONPatch(BuildKey(pi.Name), string(cPolicyByte))
	return ann, jsonPatch, err
}

//RemovePolicyJSONPatch remove JSON patch
func RemovePolicyJSONPatch(ann map[string]string, policy string) (map[string]string, []byte, error) {
	if ann == nil {
		return nil, nil, nil
	}
	jsonPatch, err := createRemoveJSONPatchKey(policy)
	return ann, jsonPatch, err
}

type patchMapValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

func createRemoveJSONPatchMap() ([]byte, error) {
	payload := []patchMapValue{{
		Op:   "remove",
		Path: "/metadata/annotations",
	}}
	return json.Marshal(payload)

}
func createAddJSONPatchMap(ann map[string]string) ([]byte, error) {

	payload := []patchMapValue{{
		Op:    "add",
		Path:  "/metadata/annotations",
		Value: ann,
	}}
	return json.Marshal(payload)
}

func createAddJSONPatch(key, value string) ([]byte, error) {

	payload := []patchStringValue{{
		Op:    "add",
		Path:  "/metadata/annotations/" + key,
		Value: value,
	}}
	return json.Marshal(payload)
}

func createReplaceJSONPatch(key, value string) ([]byte, error) {
	// if ann == nil {
	// 	ann = make(map[string]string, 0)
	// }
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/metadata/annotations/" + key,
		Value: value,
	}}
	return json.Marshal(payload)
}

func createRemoveJSONPatchKey(key string) ([]byte, error) {
	payload := []patchStringValue{{
		Op:   "remove",
		Path: "/metadata/annotations/" + key,
	}}
	return json.Marshal(payload)

}
