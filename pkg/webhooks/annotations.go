package webhooks

import (
	"encoding/json"

	"github.com/nirmata/kyverno/pkg/engine"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
)

const (
	policyAnnotation = "policies.kyverno.io"
	// lastAppliedPatches = policyAnnotation + "last-applied-patches"
)

type policyPatch struct {
	PolicyName string `json:"policyname"`
	// RulePatches []string `json:"patches"`
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

func generateAnnotationPatches(annotations map[string]string, policyResponse engine.PolicyResponse) []byte {
	if annotations == nil {
		annotations = map[string]string{}
	}
	var patchResponse response
	value := generateAnnotationsFromPolicyResponse(policyResponse)
	if value == nil {
		// no patches or error while processing patches
		return nil
	}
	if _, ok := annotations[policyAnnotation]; ok {
		// create update patch string
		patchResponse = response{
			Path:  "/metadata/annotations/" + policyAnnotation,
			Op:    "replace",
			Value: string(value),
		}
	} else {
		patchResponse = response{
			Path:  "/metadata/annotations",
			Op:    "add",
			Value: map[string]string{policyAnnotation: string(value)},
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

// func prepareAnnotationPatches(resource *unstructured.Unstructured, policyInfos []info.PolicyInfo) []byte {
// 	annots := resource.GetAnnotations()
// 	if annots == nil {
// 		annots = map[string]string{}
// 	}

// 	var patchResponse response
// 	value := annotationFromPolicies(policyInfos)
// 	if _, ok := annots[policyAnnotation]; ok {
// 		// create update patch string
// 		patchResponse = response{
// 			Op:    "replace",
// 			Path:  "/metadata/annotations/" + policyAnnotation,
// 			Value: string(value),
// 		}
// 	} else {
// 		patchResponse = response{
// 			Op:    "add",
// 			Path:  "/metadata/annotations",
// 			Value: map[string]string{policyAnnotation: string(value)},
// 		}
// 	}

// 	patchByte, _ := json.Marshal(patchResponse)

// 	// check the patch
// 	_, err := jsonpatch.DecodePatch([]byte("[" + string(patchByte) + "]"))
// 	if err != nil {
// 		glog.Errorf("Failed to make patch from annotation'%s', err: %v\n ", string(patchByte), err)
// 	}

// 	return patchByte
// }

// func annotationFromPolicies(policyInfos []info.PolicyInfo) []byte {
// 	var policyPatches []policyPatch
// 	for _, policyInfo := range policyInfos {
// 		var pp policyPatch

// 		pp.PolicyName = policyInfo.Name
// 		pp.RulePatches = annotationFromPolicy(policyInfo)
// 		policyPatches = append(policyPatches, pp)
// 	}

// 	result, _ := json.Marshal(policyPatches)

// 	return result
// }

func generateAnnotationsFromPolicyResponse(policyResponse engine.PolicyResponse) []byte {
	var rulePatches []rulePatch
	// generate annotation for each mutation JSON patch to be applied on the resource
	for _, rule := range policyResponse.Rules {
		var patchmap map[string]string
		patch := engine.JoinPatches(rule.Patches)
		if err := json.Unmarshal(patch, &patchmap); err != nil {
			glog.Errorf("Failed to parse patch bytes, err: %v\n", err)
			continue
		}

		rp := rulePatch{
			RuleName: rule.Name,
			Op:       patchmap["op"],
			Path:     patchmap["path"]}

		rulePatches = append(rulePatches, rp)
		glog.V(4).Infof("Annotation value prepared: %v\n", rulePatches)

	}
	patch, err := json.Marshal(rulePatches)
	if err != nil {
		glog.Info("failed to marshall: %v", err)
		return nil
	}
	return patch
}

// func annotationFromPolicy(policyInfo info.PolicyInfo) []rulePatch {
// 	if !policyInfo.IsSuccessful() {
// 		glog.V(2).Infof("Policy %s failed, skip preparing annotation\n", policyInfo.Name)
// 		return nil
// 	}

// 	var rulePatches []rulePatch
// 	for _, ruleInfo := range policyInfo.Rules {

// 		for _, patch := range ruleInfo.Patches {
// 			var patchmap map[string]string

// 			if err := json.Unmarshal(patch, &patchmap); err != nil {
// 				glog.Errorf("Failed to parse patch bytes, err: %v\n", err)
// 				continue
// 			}

// 			rp := rulePatch{
// 				RuleName: ruleInfo.Name,
// 				Op:       patchmap["op"],
// 				Path:     patchmap["path"]}

// 			rulePatches = append(rulePatches, rp)
// 			glog.V(4).Infof("Annotation value prepared: %v\n", rulePatches)
// 		}
// 	}

// 	return rulePatches
// }
