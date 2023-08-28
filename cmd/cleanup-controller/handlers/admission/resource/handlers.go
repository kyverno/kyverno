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

func Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	checker := manager.GetChecker()
	metadata, _, err := admissionutils.GetPartialObjectMetadatas(request.AdmissionRequest)
	gvr := admissionutils.GetGVR(request.AdmissionRequest)

	if !hasResourcePermissions(logger, gvr, checker) {
		logger.Info("resource", gvr, "doesn't have required permissions for deletion")
	}

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

func hasResourcePermissions(logger logr.Logger, resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(context.TODO(), s, resource.Group, resource.Version, resource.Resource, "", "", "watch", "list", "delete")
	if err != nil {
		logger.Error(err, "failed to check permissions")
		return false
	}
	return can
}
