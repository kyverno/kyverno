package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ws *WebhookServer) handlePolicyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var policy *kyverno.ClusterPolicy
	raw := request.Object.Raw

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	if err := json.Unmarshal(raw, &policy); err != nil {
		glog.Errorf("Failed to unmarshal policy admission request, err %v\n", err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err),
			},
		}
	}
	// Generate JSON Patches for defaults
	patches, updateMsgs := generateJSONPatchesForDefaults(policy)
	if patches != nil {
		patchType := v1beta1.PatchTypeJSONPatch
		glog.V(4).Infof("defaulted values %v policy %s", updateMsgs, policy.Name)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: strings.Join(updateMsgs, "'"),
			},
			Patch:     patches,
			PatchType: &patchType,
		}
	}
	glog.V(4).Infof("nothing to default for policy %s", policy.Name)
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func generateJSONPatchesForDefaults(policy *kyverno.ClusterPolicy) ([]byte, []string) {
	var patches [][]byte
	var updateMsgs []string

	// default 'ValidationFailureAction'
	if patch, updateMsg := defaultvalidationFailureAction(policy); patch != nil {
		patches = append(patches, patch)
		updateMsgs = append(updateMsgs, updateMsg)
	}

	return utils.JoinPatches(patches), updateMsgs
}

func defaultvalidationFailureAction(policy *kyverno.ClusterPolicy) ([]byte, string) {
	// default ValidationFailureAction to "audit" if not specified
	if policy.Spec.ValidationFailureAction == "" {
		glog.V(4).Infof("defaulting policy %s 'ValidationFailureAction' to '%s'", policy.Name, Audit)
		jsonPatch := struct {
			Path  string `json:"path"`
			Op    string `json:"op"`
			Value string `json:"value"`
		}{
			"/spec/validationFailureAction",
			"add",
			Audit, //audit
		}
		patchByte, err := json.Marshal(jsonPatch)
		if err != nil {
			glog.Errorf("failed to set default 'ValidationFailureAction' to '%s' for policy %s", Audit, policy.Name)
			return nil, ""
		}
		glog.V(4).Infof("generate JSON Patch to set default 'ValidationFailureAction' to '%s' for policy %s", Audit, policy.Name)
		return patchByte, fmt.Sprintf("default 'ValidationFailureAction' to '%s'", Audit)
	}
	return nil, ""
}
