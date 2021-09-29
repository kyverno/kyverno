package webhooks

import (
	"encoding/json"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	yamlv2 "gopkg.in/yaml.v2"
)

const (
	policyAnnotation = "policies.kyverno.io~1last-applied-patches"
	oldAnnotation    = "policies.kyverno.io~1patches"
)

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

var operationToPastTense = map[string]string{
	"add":     "added",
	"remove":  "removed",
	"replace": "replaced",
	"move":    "moved",
	"copy":    "copied",
	"test":    "tested",
}

func generateAnnotationPatches(engineResponses []*response.EngineResponse, log logr.Logger) [][]byte {
	var annotations map[string]string
	var patchBytes [][]byte

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
	value := annotationFromEngineResponses(engineResponses, log)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}

	if _, ok := annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")]; ok {
		// create update patch string
		if _, ok := annotations["policies.kyverno.io/patches"]; ok {
			patchResponse = annresponse{
				Op:   "remove",
				Path: "/metadata/annotations/policies.kyverno.io/patches",
			}
			delete(annotations, "policies.kyverno.io/patches")
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		}
		patchResponse = annresponse{
			Op:    "replace",
			Path:  "/metadata/annotations/" + policyAnnotation,
			Value: string(value),
		}
		patchByte, _ := json.Marshal(patchResponse)
		patchBytes = append(patchBytes, patchByte)
	} else {
		// mutate rule has annotation patches
		if len(annotations) > 0 {
			if _, ok := annotations["policies.kyverno.io/patches"]; ok {
				patchResponse = annresponse{
					Op:   "remove",
					Path: "/metadata/annotations/" + oldAnnotation,
				}
				delete(annotations, "policies.kyverno.io/patches")
				patchByte, _ := json.Marshal(patchResponse)
				patchBytes = append(patchBytes, patchByte)
			}
			patchResponse = annresponse{
				Op:    "add",
				Path:  "/metadata/annotations/" + policyAnnotation,
				Value: string(value),
			}
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		} else {
			// insert 'policies.kyverno.patches' entry in annotation map
			annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")] = string(value)
			patchResponse = annresponse{
				Op:    "add",
				Path:  "/metadata/annotations",
				Value: annotations,
			}
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		}
	}

	for _, patchByte := range patchBytes {
		// check the patch
		_, err := jsonpatch.DecodePatch([]byte("[" + string(patchByte) + "]"))
		if err != nil {
			log.Error(err, "failed to build JSON patch for annotation", "patch", string(patchByte))
		}
	}

	return patchBytes
}

func annotationFromEngineResponses(engineResponses []*response.EngineResponse, log logr.Logger) []byte {
	var annotationContent = make(map[string]string)
	for _, engineResponse := range engineResponses {
		if !engineResponse.IsSuccessful() {
			log.V(3).Info("skip building annotation; policy failed to apply", "policy", engineResponse.PolicyResponse.Policy.Name)
			continue
		}

		rulePatches := annotationFromPolicyResponse(engineResponse.PolicyResponse, log)
		if rulePatches == nil {
			continue
		}

		policyName := engineResponse.PolicyResponse.Policy.Name
		for _, rulePatch := range rulePatches {
			annotationContent[rulePatch.RuleName+"."+policyName+".kyverno.io"] = operationToPastTense[rulePatch.Op] + " " + rulePatch.Path
		}
	}

	// return nil if there's no patches
	// otherwise result = null, len(result) = 4
	if len(annotationContent) == 0 {
		return nil
	}

	result, _ := yamlv2.Marshal(annotationContent)

	return result
}

func annotationFromPolicyResponse(policyResponse response.PolicyResponse, log logr.Logger) []rulePatch {
	var rulePatches []rulePatch
	for _, ruleInfo := range policyResponse.Rules {
		for _, patch := range ruleInfo.Patches {
			var patchmap map[string]interface{}
			if err := json.Unmarshal(patch, &patchmap); err != nil {
				log.Error(err, "Failed to parse JSON patch bytes")
				continue
			}

			rp := rulePatch{
				RuleName: ruleInfo.Name,
				Op:       patchmap["op"].(string),
				Path:     patchmap["path"].(string)}

			rulePatches = append(rulePatches, rp)
			log.V(4).Info("annotation value prepared", "patches", rulePatches)
		}
	}
	if len(rulePatches) == 0 {
		return nil
	}
	return rulePatches
}
