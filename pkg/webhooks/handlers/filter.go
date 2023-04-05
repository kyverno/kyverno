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
		excludeUsernames := c.GetExcludedUsernames()
		for _, username := range excludeUsernames {
			if wildcard.Match(username, request.UserInfo.Username) {
				logger.V(2).Info("admission request filtered because user is excluded", "config.exlude.usernames", excludeUsernames)
				return admissionutils.ResponseSuccess(request.UID)
			}
		}
		// filter by groups
		excludeGroups := c.GetExcludedGroups()
		for _, group := range excludeGroups {
			for _, candidate := range request.UserInfo.Groups {
				if wildcard.Match(group, candidate) {
					logger.V(2).Info("admission request filtered because group is excluded", "config.exlude.groups", excludeGroups)
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by roles
		excludeRoles := c.GetExcludedRoles()
		for _, role := range excludeRoles {
			for _, candidate := range request.Roles {
				if wildcard.Match(role, candidate) {
					logger.V(2).Info("admission request filtered because role is excluded", "config.exlude.roles", excludeRoles)
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by cluster roles
		excludeClusterRoles := c.GetExcludedClusterRoles()
		for _, clusterRole := range excludeClusterRoles {
			for _, candidate := range request.ClusterRoles {
				if wildcard.Match(clusterRole, candidate) {
					logger.V(2).Info("admission request filtered because role is excluded", "config.exlude.cluster-roles", excludeClusterRoles)
					return admissionutils.ResponseSuccess(request.UID)
				}
			}
		}
		// filter by resource filters
		if c.ToFilter(request.GroupVersionKind, request.SubResource, request.Namespace, request.Name) {
			logger.V(2).Info("admission request filtered because it apears in configmap resource filters")
			return admissionutils.ResponseSuccess(request.UID)
		}
		// filter kyverno resources
		if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
			logger.V(2).Info("admission request filtered because it is for a kyverno resource")
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
			logger.V(2).Info("admission request filtered because operation is excluded")
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
		logger.V(2).Info("admission request filtered because subresource is excluded")
		return admissionutils.ResponseSuccess(request.UID)
	}
}
