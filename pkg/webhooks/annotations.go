package webhooks

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	policyAnnotation = "policies.kyverno.patches"
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

type annresponse struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

func generateAnnotationPatches(engineResponses []response.EngineResponse) []byte {
	var annotations map[string]string

	for _, er := range engineResponses {
		if ann := er.PatchedResource.GetAnnotations(); ann != nil {
			annotations = ann
			break
		}
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	var patchResponse annresponse
	value := annotationFromEngineResponses(engineResponses)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}

	if _, ok := annotations[policyAnnotation]; ok {
		// create update patch string
		patchResponse = annresponse{
			Op:    "replace",
			Path:  "/metadata/annotations/" + policyAnnotation,
			Value: string(value),
		}
	} else {
		// mutate rule has annotation patches
		if len(annotations) > 0 {
			patchResponse = annresponse{
				Op:    "add",
				Path:  "/metadata/annotations/" + policyAnnotation,
				Value: string(value),
			}
		} else {
			// insert 'policies.kyverno.patches' entry in annotation map
			annotations[policyAnnotation] = string(value)
			patchResponse = annresponse{
				Op:    "add",
				Path:  "/metadata/annotations",
				Value: annotations,
			}
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

func annotationFromEngineResponses(engineResponses []response.EngineResponse) []byte {
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
	if len(policyPatches) == 0 {
		return nil
	}

	result, _ := json.Marshal(policyPatches)

	return result
}

func annotationFromPolicyResponse(policyResponse response.PolicyResponse) []rulePatch {
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

// checkPodTemplateAnn checks if a Pod has annotation "pod-policies.kyverno.io/autogen-applied"
func checkPodTemplateAnn(resource unstructured.Unstructured) bool {
	if resource.GetKind() == "Pod" {
		ann := resource.GetAnnotations()
		if _, ok := ann[engine.PodTemplateAnnotation]; ok {
			return true
		}
	}

	return false
}
