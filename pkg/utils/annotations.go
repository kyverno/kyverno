package utils

import (
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	yamlv2 "gopkg.in/yaml.v2"
)

const (
	PolicyAnnotation      = "policies.kyverno.io/last-applied-patches"
	policyAnnotation      = "policies.kyverno.io~1last-applied-patches"
	oldAnnotation         = "policies.kyverno.io~1patches"
	ManagedByLabel        = "webhook.kyverno.io/managed-by"
	KyvernoComponentLabel = "app.kubernetes.io/component"
)

type RulePatch struct {
	RuleName string `json:"rulename"`
	Op       string `json:"op"`
	Path     string `json:"path"`
}

var OperationToPastTense = map[string]string{
	"add":     "added",
	"remove":  "removed",
	"replace": "replaced",
	"move":    "moved",
	"copy":    "copied",
	"test":    "tested",
}

func GenerateAnnotationPatches(engineResponses []*response.EngineResponse, log logr.Logger) [][]byte {
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
	var patchResponse jsonutils.PatchOperation
	value := annotationFromEngineResponses(engineResponses, log)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}
	if _, ok := annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")]; ok {
		// create update patch string
		if _, ok := annotations["policies.kyverno.io/patches"]; ok {
			patchResponse = jsonutils.NewPatchOperation("/metadata/annotations/"+oldAnnotation, "remove", nil)
			delete(annotations, "policies.kyverno.io/patches")
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		}
		patchResponse = jsonutils.NewPatchOperation("/metadata/annotations/"+policyAnnotation, "replace", string(value))
		patchByte, _ := json.Marshal(patchResponse)
		patchBytes = append(patchBytes, patchByte)
	} else {
		// mutate rule has annotation patches
		if len(annotations) > 0 {
			if _, ok := annotations["policies.kyverno.io/patches"]; ok {
				patchResponse = jsonutils.NewPatchOperation("/metadata/annotations/"+oldAnnotation, "remove", nil)
				delete(annotations, "policies.kyverno.io/patches")
				patchByte, _ := json.Marshal(patchResponse)
				patchBytes = append(patchBytes, patchByte)
			}
			patchResponse = jsonutils.NewPatchOperation("/metadata/annotations/"+policyAnnotation, "add", string(value))
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		} else {
			// insert 'policies.kyverno.patches' entry in annotation map
			annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")] = string(value)
			patchResponse = jsonutils.NewPatchOperation("/metadata/annotations", "add", annotations)
			patchByte, _ := json.Marshal(patchResponse)
			patchBytes = append(patchBytes, patchByte)
		}
	}
	for _, patchByte := range patchBytes {
		err := jsonutils.CheckPatch(patchByte)
		if err != nil {
			log.Error(err, "failed to build JSON patch for annotation", "patch", string(patchByte))
		}
	}
	return patchBytes
}

func annotationFromEngineResponses(engineResponses []*response.EngineResponse, log logr.Logger) []byte {
	annotationContent := make(map[string]string)
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
			annotationContent[rulePatch.RuleName+"."+policyName+".kyverno.io"] = OperationToPastTense[rulePatch.Op] + " " + rulePatch.Path
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

func annotationFromPolicyResponse(policyResponse response.PolicyResponse, log logr.Logger) []RulePatch {
	var RulePatches []RulePatch
	for _, ruleInfo := range policyResponse.Rules {
		for _, patch := range ruleInfo.Patches {
			var patchmap map[string]interface{}
			if err := json.Unmarshal(patch, &patchmap); err != nil {
				log.Error(err, "Failed to parse JSON patch bytes")
				continue
			}
			rp := RulePatch{
				RuleName: ruleInfo.Name,
				Op:       patchmap["op"].(string),
				Path:     patchmap["path"].(string),
			}
			RulePatches = append(RulePatches, rp)
			log.V(4).Info("annotation value prepared", "patches", RulePatches)
		}
	}
	if len(RulePatches) == 0 {
		return nil
	}
	return RulePatches
}
