package handlers

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (h AdmissionHandler) WithProtection(enabled bool) AdmissionHandler {
	if !enabled {
		return h
	}
	return withProtection(h)
}

func withProtection(inner AdmissionHandler) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		newResource, oldResource, err := utils.ExtractResources(nil, request)
		if err != nil {
			logger.Error(err, "Failed to extract resources")
			return admissionutils.Response(err)
		}
		for _, resource := range []unstructured.Unstructured{newResource, oldResource} {
			resLabels := resource.GetLabels()
			if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp {
				if request.UserInfo.Username != fmt.Sprintf("system:serviceaccount:%s:%s", config.KyvernoNamespace(), config.KyvernoServiceAccountName()) {
					logger.Info("Access to the resource not authorized, this is a kyverno managed resource and should be altered only by kyverno")
					return admissionutils.Response(errors.New("A kyverno managed resource can only be modified by kyverno"))
				}
			}
		}
		return inner(logger, request, startTime)
	}
}
