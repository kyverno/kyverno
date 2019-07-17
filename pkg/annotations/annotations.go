package annotations

import (
	"encoding/json"

	"github.com/golang/glog"

	"github.com/nirmata/kyverno/pkg/info"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//Policy information for annotations
type Policy struct {
	Status string `json:"status"`
	Rules  []Rule `json:"rules,omitempty"`
}

//Rule information for annotations
type Rule struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Type    string `json:"type"`
	Changes string `json:"changes"`
}

func getStatus(status bool) string {
	if status {
		return "Success"
	}
	return "Failure"
}
func getRules(rules []*info.RuleInfo) []Rule {
	var annrules []Rule
	for _, r := range rules {
		annrule := Rule{Name: r.Name,
			Status: getStatus(r.IsSuccessful()),
			Type:   r.RuleType.String()}
		//TODO: add mutation changes in policyInfo and in annotation
		annrules = append(annrules, annrule)
	}
	return annrules
}

func (p *Policy) updatePolicy(obj *Policy, ruleType info.RuleType) {
	p.Status = obj.Status
	p.updatePolicyRules(obj.Rules, ruleType)
}

// Update rules of a given type
func (p *Policy) updatePolicyRules(rules []Rule, ruleType info.RuleType) {
	var updatedRules []Rule
	//TODO: check the selecting update add advantage
	// filter rules for different type
	for _, r := range rules {
		if r.Type != ruleType.String() {
			updatedRules = append(updatedRules, r)
		}
	}
	// Add rules for current type
	updatedRules = append(updatedRules, rules...)
	// set the rule
	p.Rules = updatedRules
}

// func (p *Policy) containsPolicyRules(rules []Rule, ruleType info.RuleType) {
// 	for _, r := range rules {
// 	}
// }
func newAnnotationForPolicy(pi *info.PolicyInfo) *Policy {
	return &Policy{Status: getStatus(pi.IsSuccessful()),
		Rules: getRules(pi.Rules)}
}

//AddPolicy will add policy annotation if not present or update if present
func AddPolicy(obj *unstructured.Unstructured, pi *info.PolicyInfo, ruleType info.RuleType) error {
	PolicyObj := newAnnotationForPolicy(pi)
	// get annotation
	ann := obj.GetAnnotations()
	// check if policy already has annotation
	cPolicy, ok := ann[pi.Name]
	if !ok {
		PolicyByte, err := json.Marshal(PolicyObj)
		if err != nil {
			return err
		}
		// insert policy information
		ann[pi.Name] = string(PolicyByte)
		// set annotation back to unstr
		obj.SetAnnotations(ann)
		return nil
	}
	cPolicyObj := Policy{}
	err := json.Unmarshal([]byte(cPolicy), &cPolicyObj)
	// update policy information inside the annotation
	// 1> policy status
	// 2> rule (name, status,changes,type)
	cPolicyObj.updatePolicy(PolicyObj, ruleType)
	if err != nil {
		return err
	}
	cPolicyByte, err := json.Marshal(cPolicyObj)
	if err != nil {
		return err
	}
	// update policy information
	ann[pi.Name] = string(cPolicyByte)
	// set annotation back to unstr
	obj.SetAnnotations(ann)
	return nil
}

//RemovePolicy to remove annotations fro
func RemovePolicy(obj *unstructured.Unstructured, policy string) {
	// get annotations
	ann := obj.GetAnnotations()
	delete(ann, policy)
	// set annotation back to unstr
	obj.SetAnnotations(ann)
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
	if annotations, ok := meta["annotations"].(map[string]string); ok {
		return annotations
	}
	return nil
}

//AddPolicyJSONPatch generate JSON Patch to add policy informatino JSON patch
func AddPolicyJSONPatch(ann map[string]string, pi *info.PolicyInfo, ruleType info.RuleType) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string, 0)
	}
	PolicyObj := newAnnotationForPolicy(pi)
	cPolicy, ok := ann[pi.Name]
	if !ok {
		PolicyByte, err := json.Marshal(PolicyObj)
		if err != nil {
			return nil, err
		}
		// insert policy information
		ann[pi.Name] = string(PolicyByte)
		// create add JSON patch
		return createAddJSONPatch(ann)
	}
	cPolicyObj := Policy{}
	err := json.Unmarshal([]byte(cPolicy), &cPolicyObj)
	// update policy information inside the annotation
	// 1> policy status
	// 2> rule (name, status,changes,type)
	cPolicyObj.updatePolicy(PolicyObj, ruleType)
	if err != nil {
		return nil, err
	}
	cPolicyByte, err := json.Marshal(cPolicyObj)
	if err != nil {
		return nil, err
	}
	// update policy information
	ann[pi.Name] = string(cPolicyByte)
	// create update JSON patch
	return createReplaceJSONPatch(ann)
}

//RemovePolicyJSONPatch remove JSON patch
func RemovePolicyJSONPatch(ann map[string]string, policy string) ([]byte, error) {
	if ann == nil {
		return nil, nil
	}
	delete(ann, policy)
	if len(ann) == 0 {
		return createRemoveJSONPatch(ann)
	}
	return createReplaceJSONPatch(ann)
}

type patchMapValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

func createRemoveJSONPatch(ann map[string]string) ([]byte, error) {
	payload := []patchMapValue{{
		Op:   "remove",
		Path: "/metadata/annotations",
	}}
	return json.Marshal(payload)

}
func createAddJSONPatch(ann map[string]string) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string, 0)
	}
	payload := []patchMapValue{{
		Op:    "add",
		Path:  "/metadata/annotations",
		Value: ann,
	}}
	return json.Marshal(payload)
}

func createReplaceJSONPatch(ann map[string]string) ([]byte, error) {
	if ann == nil {
		ann = make(map[string]string, 0)
	}
	payload := []patchMapValue{{
		Op:    "replace",
		Path:  "/metadata/annotations",
		Value: ann,
	}}
	return json.Marshal(payload)
}
