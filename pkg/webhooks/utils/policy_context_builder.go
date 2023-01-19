package utils

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type PolicyContextBuilder interface {
	Build(*admissionv1.AdmissionRequest) (*engine.PolicyContext, error)
}

type policyContextBuilder struct {
	configuration          config.Configuration
	client                 dclient.Interface
	rbLister               rbacv1listers.RoleBindingLister
	crbLister              rbacv1listers.ClusterRoleBindingLister
	informerCacheResolvers resolvers.ConfigmapResolver
	polexLister            engine.PolicyExceptionLister
}

func NewPolicyContextBuilder(
	configuration config.Configuration,
	client dclient.Interface,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
	informerCacheResolvers resolvers.ConfigmapResolver,
	polexLister engine.PolicyExceptionLister,
) PolicyContextBuilder {
	return &policyContextBuilder{
		configuration:          configuration,
		client:                 client,
		rbLister:               rbLister,
		crbLister:              crbLister,
		informerCacheResolvers: informerCacheResolvers,
		polexLister:            polexLister,
	}
}

func (b *policyContextBuilder) Build(request *admissionv1.AdmissionRequest) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}
	if roles, clusterRoles, err := userinfo.GetRoleRef(b.rbLister, b.crbLister, request, b.configuration); err != nil {
		return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
	} else {
		userRequestInfo.Roles = roles
		userRequestInfo.ClusterRoles = clusterRoles
	}
	return engine.NewPolicyContextFromAdmissionRequest(request, userRequestInfo, b.configuration, b.client, b.informerCacheResolvers, b.polexLister)
}
