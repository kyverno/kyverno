package utils

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type PolicyContextBuilder interface {
	Build(*admissionv1.AdmissionRequest, ...kyvernov1.PolicyInterface) (*engine.PolicyContext, error)
}

func newVariablesContext(request *admissionv1.AdmissionRequest, userRequestInfo *kyvernov1beta1.RequestInfo) (enginectx.Interface, error) {
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(request); err != nil {
		return nil, errors.Wrap(err, "failed to load incoming request in context")
	}
	if err := ctx.AddUserInfo(*userRequestInfo); err != nil {
		return nil, errors.Wrap(err, "failed to load userInfo in context")
	}
	if err := ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username); err != nil {
		return nil, errors.Wrap(err, "failed to load service account in context")
	}
	return ctx, nil
}

type policyContextBuilder struct {
	configuration config.Configuration
	client        dclient.Interface
	rbLister      rbacv1listers.RoleBindingLister
	crbLister     rbacv1listers.ClusterRoleBindingLister
}

func NewPolicyContextBuilder(
	configuration config.Configuration,
	client dclient.Interface,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
) PolicyContextBuilder {
	return &policyContextBuilder{
		configuration: configuration,
		client:        client,
		rbLister:      rbLister,
		crbLister:     crbLister,
	}
}

func (b *policyContextBuilder) Build(request *admissionv1.AdmissionRequest, policies ...kyvernov1.PolicyInterface) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}
	if roles, clusterRoles, err := userinfo.GetRoleRef(b.rbLister, b.crbLister, request, b.configuration); err != nil {
		return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
	} else {
		userRequestInfo.Roles = roles
		userRequestInfo.ClusterRoles = clusterRoles
	}
	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy rule context")
	}
	newResource, oldResource, err := utils.ExtractResources(nil, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource")
	}
	if err := ctx.AddImageInfos(&newResource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}
	policyContext := &engine.PolicyContext{
		NewResource:         newResource,
		OldResource:         oldResource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    b.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: b.configuration.ToFilter,
		JSONContext:         ctx,
		Client:              b.client,
		AdmissionOperation:  true,
	}
	return policyContext, nil
}
