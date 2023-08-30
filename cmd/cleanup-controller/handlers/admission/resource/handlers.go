package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	manager "github.com/kyverno/kyverno/pkg/controllers/ttl"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/resource"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type validationHandlers struct {
	checker checker.AuthChecker
}

func New(checker checker.AuthChecker) *validationHandlers {
	return &validationHandlers{
		checker: checker,
	}
}

func (h *validationHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	metadata, _, err := admissionutils.GetPartialObjectMetadatas(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal metadatas from admission request")
		return admissionutils.ResponseSuccess(request.UID, err.Error())
	}
	if !manager.HasResourcePermissions(logger, schema.GroupVersionResource(request.AdmissionRequest.Resource), h.checker) {
		logger.Info("doesn't have required permissions for deletion", "gvr", request.AdmissionRequest.Resource)
	}
	if err := validation.ValidateTtlLabel(ctx, metadata); err != nil {
		logger.Error(err, "metadatas validation errors")
		return admissionutils.ResponseSuccess(request.UID, fmt.Sprintf("cleanup.kyverno.io/ttl label value cannot be parsed as any recognizable format (%s)", err.Error()))
	}
	return admissionutils.ResponseSuccess(request.UID)
}
