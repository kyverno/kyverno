package utils

import (
	"fmt"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/userinfo"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type PolicyContextBuilder interface {
	Build(*admissionv1.AdmissionRequest) (*engine.PolicyContext, error)
}

type policyContextBuilder struct {
	configuration config.Configuration
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
		rbLister:      rbLister,
		crbLister:     crbLister,
	}
}

func (b *policyContextBuilder) Build(request *admissionv1.AdmissionRequest) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}
	if roles, clusterRoles, err := userinfo.GetRoleRef(b.rbLister, b.crbLister, request, b.configuration); err != nil {
		return nil, fmt.Errorf("failed to fetch RBAC information for request: %w", err)
	} else {
		userRequestInfo.Roles = roles
		userRequestInfo.ClusterRoles = clusterRoles
	}
	return engine.NewPolicyContextFromAdmissionRequest(request, userRequestInfo, b.configuration)
}
