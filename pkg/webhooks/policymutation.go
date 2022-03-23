package webhooks

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	logr "github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policymutation"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ws *WebhookServer) policyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policy mutation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	var policy *kyverno.ClusterPolicy
	raw := request.Object.Raw

	if err := json.Unmarshal(raw, &policy); err != nil {
		logger.Error(err, "failed to unmarshal policy admission request")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err),
			},
		}
	}

	if request.Operation == v1beta1.Update {
		admissionResponse := hasPolicyChanged(policy, request.OldObject.Raw, logger)
		if admissionResponse != nil {
			logger.V(4).Info("skip policy mutation on status update")
			return admissionResponse
		}
	}

	startTime := time.Now()
	logger.V(3).Info("start policy change mutation")
	defer logger.V(3).Info("finished policy change mutation", "time", time.Since(startTime).String())

	// Generate JSON Patches for defaults
	patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger)
	if len(patches) != 0 {
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

func hasPolicyChanged(policy *kyverno.ClusterPolicy, oldRaw []byte, logger logr.Logger) *v1beta1.AdmissionResponse {
	var oldPolicy *kyverno.ClusterPolicy
	if err := json.Unmarshal(oldRaw, &oldPolicy); err != nil {
		logger.Error(err, "failed to unmarshal old policy admission request")
		return &v1beta1.AdmissionResponse{
			Allowed: true,
			Result: &metav1.Status{
				Message: fmt.Sprintf("failed to validate policy, check kyverno controller logs for details: %v", err),
			},
		}

	}

	if isStatusUpdate(oldPolicy, policy) {
		return &v1beta1.AdmissionResponse{Allowed: true}
	}

	return nil
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
