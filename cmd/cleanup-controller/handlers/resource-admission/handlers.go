package resourceadmission

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/ttl-label"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

func Validate(_ context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	logger.Info("triggered the label validator")
	ttlLabel, err := admissionutils.GetTtlLabel(request.AdmissionRequest.Object.Raw)
	if err != nil {
		logger.Error(err, "failed to get the ttl label")
		return admissionutils.ResponseSuccess(request.UID, err.Error())
	}

	if request.Operation == admissionv1.Update {
		ttlLabel, err = admissionutils.GetTtlLabel(request.AdmissionRequest.Object.Raw)
		if err != nil {
			logger.Error(err, "failed to get the ttl label")
			return admissionutils.ResponseSuccess(request.UID, err.Error())
		}
	}

	if ttlLabel != "" {
		if err := validation.Validate(ttlLabel); err != nil {
			logger.Error(err, "failed to unmarshal the ttl label value")
			return admissionutils.ResponseSuccess(request.UID, err.Error())
		}
	}
	return admissionutils.ResponseSuccess(request.UID)
}
