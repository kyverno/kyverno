package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
)

func (inner AdmissionHandler) withGenerateProvenanceProtection() AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		if strings.HasPrefix(request.UserInfo.Username, kyvernoUsernamePrefix) {
			return inner(ctx, logger, request, startTime)
		}
		if request.Operation != admissionv1.Create && request.Operation != admissionv1.Update {
			return inner(ctx, logger, request, startTime)
		}
		newResource, oldResource, err := admissionutils.ExtractResources(nil, request.AdmissionRequest)
		if err != nil {
			logger.Error(err, "failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		newStamp, newStampSet := newResource.GetAnnotations()[provenance.Annotation]
		oldStamp, oldStampSet := oldResource.GetAnnotations()[provenance.Annotation]
		if newStampSet != oldStampSet || newStamp != oldStamp {
			logger.V(2).Info("access to Kyverno generate provenance is not authorized")
			return admissionutils.Response(request.UID, errors.New("Kyverno generate provenance can only be set by Kyverno"))
		}
		return inner(ctx, logger, request, startTime)
	}
}
