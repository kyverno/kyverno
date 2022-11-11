package handlers

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	admissionv1 "k8s.io/api/admission/v1"
)

func Verify() AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if request.Name != "kyverno-health" || request.Namespace != config.KyvernoNamespace() {
			return admissionutils.ResponseSuccess()
		}
		patch := jsonutils.NewPatchOperation("/metadata/annotations/"+"kyverno.io~1last-request-time", "replace", time.Now().Format(time.RFC3339))
		bytes, err := patch.ToPatchBytes()
		if err != nil {
			logger.Error(err, "failed to build patch bytes")
			return admissionutils.Response(err)
		}
		return admissionutils.MutationResponse(bytes)
	}
}
