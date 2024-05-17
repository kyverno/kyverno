package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tracing"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func (inner AdmissionHandler) WithFilter(configuration config.Configuration) AdmissionHandler {
	return inner.withFilter(configuration).WithTrace("FILTER")
}

func (inner AdmissionHandler) WithOperationFilter(operations ...admissionv1.Operation) AdmissionHandler {
	return inner.withOperationFilter(operations...).WithTrace("OPERATION")
}

func (inner AdmissionHandler) WithSubResourceFilter(subresources ...string) AdmissionHandler {
	return inner.withSubResourceFilter(subresources...).WithTrace("SUBRESOURCE")
}

func filtered(ctx context.Context, logger logr.Logger, request AdmissionRequest, message string, keysAndValues ...interface{}) AdmissionResponse {
	logger.V(4).Info(message, keysAndValues...)
	tracing.SetAttributes(ctx, tracing.RequestFilteredKey.Bool(true))
	return admissionutils.ResponseSuccess(request.UID)
}

func (inner AdmissionHandler) withFilter(c config.Configuration) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		// filter by exclusions/inclusions
		if c.IsExcluded(request.UserInfo.Username, request.UserInfo.Groups, request.Roles, request.ClusterRoles) {
			return filtered(ctx, logger, request, "admission request filtered")
		}
		// filter by resource filters
		if c.ToFilter(request.GroupVersionKind, request.SubResource, request.Namespace, request.Name) {
			return filtered(ctx, logger, request, "admission request filtered because it apears in configmap resource filters")
		}
		// filter kyverno resources
		if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
			return filtered(ctx, logger, request, "admission request filtered because it is for a kyverno resource")
		}
		return inner(ctx, logger, request, startTime)
	}
}

func (inner AdmissionHandler) withOperationFilter(operations ...admissionv1.Operation) AdmissionHandler {
	allowed := sets.New[string]()
	for _, operation := range operations {
		allowed.Insert(string(operation))
	}
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		if allowed.Has(string(request.Operation)) {
			return inner(ctx, logger, request, startTime)
		}
		return filtered(ctx, logger, request, "admission request filtered because operation is excluded")
	}
}

func (inner AdmissionHandler) withSubResourceFilter(subresources ...string) AdmissionHandler {
	allowed := sets.New(subresources...)
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		if request.SubResource == "" || allowed.Has(request.SubResource) {
			return inner(ctx, logger, request, startTime)
		}
		return filtered(ctx, logger, request, "admission request filtered because subresource is excluded")
	}
}
