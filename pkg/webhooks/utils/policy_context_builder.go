package utils

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
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

func checkForRBACInfo(rule kyvernov1.Rule) bool {
	if len(rule.MatchResources.Roles) > 0 || len(rule.MatchResources.ClusterRoles) > 0 || len(rule.ExcludeResources.Roles) > 0 || len(rule.ExcludeResources.ClusterRoles) > 0 {
		return true
	}
	if len(rule.MatchResources.All) > 0 {
		for _, rf := range rule.MatchResources.All {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.MatchResources.Any) > 0 {
		for _, rf := range rule.MatchResources.Any {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.ExcludeResources.All) > 0 {
		for _, rf := range rule.ExcludeResources.All {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.ExcludeResources.Any) > 0 {
		for _, rf := range rule.ExcludeResources.Any {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	return false
}

func containsRBACInfo(policies ...kyvernov1.PolicyInterface) bool {
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			if checkForRBACInfo(rule) {
				return true
			}
		}
	}
	return false
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
	if containsRBACInfo(policies...) {
		var err error
		userRequestInfo.Roles, userRequestInfo.ClusterRoles, err = userinfo.GetRoleRef(b.rbLister, b.crbLister, request, b.configuration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
		}
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
