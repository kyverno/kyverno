package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (inner AdmissionHandler) WithProtection(enabled bool, enablePolex bool, polexNamespace string) AdmissionHandler {
	if !enabled {
		return inner
	}
	return inner.withProtection(enablePolex, polexNamespace).WithTrace("PROTECT")
}

func (inner AdmissionHandler) withProtection(enablePolex bool, polexNamespace string) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		newResource, oldResource, err := admissionutils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "Failed to extract resources")
			return admissionutils.Response(request.UID, err)
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
			resLabels := resource.GetLabels()
			if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp {
				if request.UserInfo.Username != fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName()) {
					logger.Info("Access to the resource not authorized, this is a kyverno managed resource and should be altered only by kyverno")
					return admissionutils.ResponseSuccess(request.UID, "A kyverno managed resource can only be modified by kyverno")
				}
			}
		}

		return inner(ctx, logger, request, startTime)
	}
}
