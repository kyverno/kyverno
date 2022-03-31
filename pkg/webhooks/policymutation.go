package webhooks

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policymutation"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	v1beta1 "k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) policyMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	logger := ws.log.WithValues("action", "policy mutation", "uid", request.UID, "kind", request.Kind, "namespace", request.Namespace, "name", request.Name, "operation", request.Operation, "gvk", request.Kind.String())
	policy, oldPolicy, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err))
	}
	if oldPolicy != nil && isStatusUpdate(oldPolicy, policy) {
		logger.V(4).Info("skip policy mutation on status update")
		return admissionutils.Response(true)
	}
	startTime := time.Now()
	logger.V(3).Info("start policy change mutation")
	defer logger.V(3).Info("finished policy change mutation", "time", time.Since(startTime).String())
	// Generate JSON Patches for defaults
	if patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger); len(patches) != 0 {
		return admissionutils.ResponseWithMessageAndPatch(true, strings.Join(updateMsgs, "'"), patches)
	}
	return admissionutils.Response(true)
}

func isStatusUpdate(old, new kyverno.PolicyInterface) bool {
	if !reflect.DeepEqual(old.GetAnnotations(), new.GetAnnotations()) {
		return false
	}
	if !reflect.DeepEqual(old.GetLabels(), new.GetLabels()) {
		return false
	}
	if !reflect.DeepEqual(old.GetSpec(), new.GetSpec()) {
		return false
	}
	return true
}
