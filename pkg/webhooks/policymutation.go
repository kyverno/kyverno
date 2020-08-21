package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/policymutation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ws *WebhookServer) policyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policymutation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	var policy *kyverno.ClusterPolicy
	raw := request.Object.Raw

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	if err := json.Unmarshal(raw, &policy); err != nil {
		logger.Error(err, "failed to unmarshall policy admission request")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err),
			},
		}
	}
	// Generate JSON Patches for defaults
	patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger)
	if patches != nil {
		patchType := v1beta1.PatchTypeJSONPatch
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: strings.Join(updateMsgs, "'"),
			},
			Patch:     patches,
			PatchType: &patchType,
		}
	}
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
