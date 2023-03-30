package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
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

func (inner AdmissionHandler) withFilter(c config.Configuration) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		// filter by username
		for _, username := range c.GetExcludedUsernames() {
			if wildcard.Match(username, request.UserInfo.Username) {
				return nil
			}
		}
		// filter by groups
		for _, group := range c.GetExcludedGroups() {
			for _, candidate := range request.UserInfo.Groups {
				if wildcard.Match(group, candidate) {
					return nil
				}
			}
		}
		// filter by resource filters
		if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			return nil
		}
		// filter kyverno resources
		if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
			return nil
		}
		return inner(ctx, logger, request, startTime)
	}
}

func (inner AdmissionHandler) withOperationFilter(operations ...admissionv1.Operation) AdmissionHandler {
	allowed := sets.New[string]()
	for _, operation := range operations {
		allowed.Insert(string(operation))
	}
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if allowed.Has(string(request.Operation)) {
			return inner(ctx, logger, request, startTime)
		}
		return nil
	}
}

func (inner AdmissionHandler) withSubResourceFilter(subresources ...string) AdmissionHandler {
	allowed := sets.New(subresources...)
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if request.SubResource == "" || allowed.Has(request.SubResource) {
			return inner(ctx, logger, request, startTime)
		}
		return nil
	}
}
