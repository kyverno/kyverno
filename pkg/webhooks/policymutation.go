package webhooks

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policymutation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ws *WebhookServer) policyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policymutation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation)
	var policy *kyverno.ClusterPolicy
	raw := request.Object.Raw

	//TODO: can this happen? wont this be picked by OpenAPI spec schema ?
	if err := json.Unmarshal(raw, &policy); err != nil {
		logger.Error(err, "failed to unmarshal policy admission request")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err),
			},
		}
	}

	// Generate JSON Patches for defaults
	patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger)
	logger.Info("patches", "len", len(patches), "object", string(patches))
	if len(patches) != 0 {
		patchType := v1beta1.PatchTypeJSONPatch
		logger.Info("patches patched")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: strings.Join(updateMsgs, "'"),
			},
			Patch:     patches,
			PatchType: &patchType,
		}
	}

	logger.Info("patches empty")
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}

func hasPolicyChanged(policy *kyverno.ClusterPolicy, oldRaw []byte) (bool, error) {
	var oldPolicy *kyverno.ClusterPolicy
	if err := json.Unmarshal(oldRaw, &oldPolicy); err != nil {
		return false, err
	}

	return !isStatusUpdate(oldPolicy, policy), nil
}

func isStatusUpdate(old, new *kyverno.ClusterPolicy) bool {
	if !reflect.DeepEqual(old.GetAnnotations(), new.GetAnnotations()) {
		return false
	}

	if !reflect.DeepEqual(old.GetLabels(), new.GetLabels()) {
		return false
	}

	if !reflect.DeepEqual(old.Spec, new.Spec) {
		return false
	}

	return true
}
