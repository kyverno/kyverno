package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/resource"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

func Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	metadata, _, err := admissionutils.GetPartialObjectMetadatas(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal metadatas from admission request")
		return admissionutils.ResponseSuccess(request.UID, err.Error())
	}
	if err := validation.ValidateTtlLabel(ctx, metadata); err != nil {
		logger.Error(err, "metadatas validation errors")
		return admissionutils.ResponseSuccess(request.UID, fmt.Sprintf("cleanup.kyverno.io/ttl label value cannot be parsed as any recognizable format (%s)", err.Error()))
	}
	return admissionutils.ResponseSuccess(request.UID)
}
