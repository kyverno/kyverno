package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
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
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		// filter by username
		for _, username := range c.GetExcludedUsernames() {
			if wildcard.Match(username, request.UserInfo.Username) {
				return admissionutils.ResponseSuccess(request.UID)
			}
		}
		// filter by groups
		for _, group := range c.GetExcludedGroups() {
			for _, candidate := range request.UserInfo.Groups {
				if wildcard.Match(group, candidate) {
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by roles
		for _, role := range c.GetExcludedRoles() {
			for _, candidate := range request.Roles {
				if wildcard.Match(role, candidate) {
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by cluster roles
		for _, clusterRole := range c.GetExcludedClusterRoles() {
			for _, candidate := range request.ClusterRoles {
				if wildcard.Match(clusterRole, candidate) {
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by resource filters
		if c.ToFilter(request.GroupVersionKind, request.SubResource, request.Namespace, request.Name) {
			return admissionutils.ResponseSuccess(request.UID)
		}
		// filter kyverno resources
		if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
			return admissionutils.ResponseSuccess(request.UID)
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
		return admissionutils.ResponseSuccess(request.UID)
	}
}

func (inner AdmissionHandler) withSubResourceFilter(subresources ...string) AdmissionHandler {
	allowed := sets.New(subresources...)
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		if request.SubResource == "" || allowed.Has(request.SubResource) {
			return inner(ctx, logger, request, startTime)
		}
		return admissionutils.ResponseSuccess(request.UID)
	}
}
