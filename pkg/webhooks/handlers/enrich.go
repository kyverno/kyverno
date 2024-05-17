package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/userinfo"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"k8s.io/apimachinery/pkg/runtime/schema"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

func (inner AdmissionHandler) WithRoles(
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
) AdmissionHandler {
	return inner.withRoles(rbLister, crbLister).WithTrace("ROLES")
}

func (inner AdmissionHandler) WithTopLevelGVK(
	client dclient.IDiscovery,
) AdmissionHandler {
	return inner.withTopLevelGVK(client).WithTrace("GVK")
}

func (inner AdmissionHandler) withRoles(
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		roles, clusterRoles, err := userinfo.GetRoleRef(rbLister, crbLister, request.UserInfo)
		if err != nil {
			logger.Error(err, "failed to get roles/cluster roles from user infos")
			return admissionutils.Response(request.UID, err)
		}
		request.Roles = roles
		request.ClusterRoles = clusterRoles
		logger = logger.WithValues(
			"roles", roles,
			"clusterroles", clusterRoles,
		)
		return inner(ctx, logger, request, startTime)
	}
}

func (inner AdmissionHandler) withTopLevelGVK(
	client dclient.IDiscovery,
) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		gvk, err := client.GetGVKFromGVR(schema.GroupVersionResource(request.Resource))
		if err != nil {
			logger.Error(err, "failed to get top level GVK from GVR")
			return admissionutils.Response(request.UID, err)
		}
		request.GroupVersionKind = gvk
		logger = logger.WithValues(
			"resource.gvk", gvk,
		)
		return inner(ctx, logger, request, startTime)
	}
}
