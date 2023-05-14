package utils

import (
	"strings"

	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/mattbaird/jsonpatch"
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

func GenerateAnnotationPatches(engineResponses []engineapi.EngineResponse, log logr.Logger) []jsonpatch.JsonPatchOperation {
	var annotations map[string]string
	var patchBytes []jsonpatch.JsonPatchOperation
	for _, er := range engineResponses {
		if ann := er.PatchedResource.GetAnnotations(); ann != nil {
			annotations = ann
			break
		}
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}
	var patchResponse jsonpatch.JsonPatchOperation
	value := annotationFromEngineResponses(engineResponses, log)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}
	if _, ok := annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")]; ok {
		// create update patch string
		if _, ok := annotations["policies.kyverno.io/patches"]; ok {
			patchResponse = jsonpatch.JsonPatchOperation{
				Operation: "remove",
				Path:      "/metadata/annotations/" + oldAnnotation,
				Value:     nil,
			}
			delete(annotations, "policies.kyverno.io/patches")
			patchBytes = append(patchBytes, patchResponse)
		}
		patchResponse = jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      "/metadata/annotations/" + policyAnnotation,
			Value:     string(value),
		}
		patchBytes = append(patchBytes, patchResponse)
	} else {
		// mutate rule has annotation patches
		if len(annotations) > 0 {
			if _, ok := annotations["policies.kyverno.io/patches"]; ok {
				patchResponse = jsonpatch.JsonPatchOperation{
					Operation: "remove",
					Path:      "/metadata/annotations/" + oldAnnotation,
					Value:     nil,
				}
				delete(annotations, "policies.kyverno.io/patches")
				patchBytes = append(patchBytes, patchResponse)
			}
			patchResponse = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/annotations/" + policyAnnotation,
				Value:     string(value),
			}
			patchBytes = append(patchBytes, patchResponse)
		} else {
			// insert 'policies.kyverno.patches' entry in annotation map
			annotations[strings.ReplaceAll(policyAnnotation, "~1", "/")] = string(value)
			patchResponse = jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/metadata/annotations",
				Value:     annotations,
			}
			patchBytes = append(patchBytes, patchResponse)
		}
	}
	return patchBytes
}

func annotationFromEngineResponses(engineResponses []engineapi.EngineResponse, log logr.Logger) []byte {
	annotationContent := make(map[string]string)
	for _, engineResponse := range engineResponses {
		if !engineResponse.IsSuccessful() {
			log.V(3).Info("skip building annotation; policy failed to apply", "policy", engineResponse.Policy().GetName())
			continue
		}
		rulePatches := annotationFromPolicyResponse(engineResponse.PolicyResponse, log)
		if rulePatches == nil {
			continue
		}
		policyName := engineResponse.Policy().GetName()
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

func annotationFromPolicyResponse(policyResponse engineapi.PolicyResponse, log logr.Logger) []RulePatch {
	var RulePatches []RulePatch
	for _, ruleInfo := range policyResponse.Rules {
		for _, patch := range ruleInfo.Patches() {
			rp := RulePatch{
				RuleName: ruleInfo.Name(),
				Op:       patch.Operation,
				Path:     patch.Path,
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
