package webhooks

import (
	"encoding/json"

	"github.com/nirmata/kyverno/pkg/engine"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
)

const (
	policyAnnotation = "policies.kyverno.io"
)

type policyPatch struct {
	PolicyName  string      `json:"policyname"`
	RulePatches interface{} `json:"patches"`
}

type rulePatch struct {
	RuleName string `json:"rulename"`
	Op       string `json:"op"`
	Path     string `json:"path"`
}

type response struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func generateAnnotationPatches(annotations map[string]string, engineResponses []engine.EngineResponseNew) []byte {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	var patchResponse response
	value := annotationFromPolicyResponses(engineResponses)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}

	if _, ok := annotations[policyAnnotation]; ok {
		// create update patch string
		patchResponse = response{
			Op:    "replace",
			Path:  "/metadata/annotations/" + policyAnnotation,
			Value: string(value),
		}
	} else {
		// insert 'policies.kyverno.io' entry in annotation map
		annotations[policyAnnotation] = string(value)
		patchResponse = response{
			Op:    "add",
			Path:  "/metadata/annotations",
			Value: annotations,
		}
	}

	patchByte, _ := json.Marshal(patchResponse)

	// check the patch
	_, err := jsonpatch.DecodePatch([]byte("[" + string(patchByte) + "]"))
	if err != nil {
		glog.Errorf("Failed to make patch from annotation'%s', err: %v\n ", string(patchByte), err)
	}

	return patchByte
}

func annotationFromPolicyResponses(engineResponses []engine.EngineResponseNew) []byte {
	var policyPatches []policyPatch
	for _, engineResponse := range engineResponses {
		if !engineResponse.IsSuccesful() {
			glog.V(3).Infof("Policy %s failed, skip preparing annotation\n", engineResponse.PolicyResponse.Policy)
			continue
		}

		var pp policyPatch
		rulePatches := annotationFromPolicyResponse(engineResponse.PolicyResponse)
		if rulePatches == nil {
			continue
		}

		pp.RulePatches = rulePatches
		pp.PolicyName = engineResponse.PolicyResponse.Policy
		policyPatches = append(policyPatches, pp)
	}

	// return nil if there's no patches
	// otherwise result = null, len(result) = 4
	if policyPatches == nil {
		return nil
	}

	result, _ := json.Marshal(policyPatches)

	return result
}

func annotationFromPolicyResponse(policyResponse engine.PolicyResponse) []rulePatch {
	var rulePatches []rulePatch
	for _, ruleInfo := range policyResponse.Rules {
		for _, patch := range ruleInfo.Patches {
			var patchmap map[string]interface{}
			if err := json.Unmarshal(patch, &patchmap); err != nil {
				glog.Errorf("Failed to parse patch bytes, err: %v\n", err)
				continue
			}

			rp := rulePatch{
				RuleName: ruleInfo.Name,
				Op:       patchmap["op"].(string),
				Path:     patchmap["path"].(string)}

			rulePatches = append(rulePatches, rp)
			glog.V(4).Infof("Annotation value prepared: %v\n", rulePatches)
		}
	}

	if len(rulePatches) == 0 {
		return nil
	}

	return rulePatches
}
