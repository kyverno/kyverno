package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
)

func Verify(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
	if request.Name != "kyverno-health" || request.Namespace != config.KyvernoNamespace() {
		return admissionutils.ResponseSuccess(request.UID)
	}
	patch := jsonutils.NewPatchOperation("/metadata/annotations/"+"kyverno.io~1last-request-time", "replace", time.Now().Format(time.RFC3339))
	bytes, err := patch.ToPatchBytes()
	if err != nil {
		logger.Error(err, "failed to build patch bytes")
		return admissionutils.Response(request.UID, err)
	}
	return admissionutils.MutationResponse(request.UID, bytes)
}
