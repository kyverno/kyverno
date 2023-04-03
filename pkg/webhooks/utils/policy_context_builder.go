package utils

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	admissionv1 "k8s.io/api/admission/v1"
)

type PolicyContextBuilder interface {
	Build(admissionv1.AdmissionRequest, []string, []string) (*engine.PolicyContext, error)
}

type policyContextBuilder struct {
	configuration config.Configuration
	client        dclient.Interface
}

func NewPolicyContextBuilder(
	configuration config.Configuration,
	client dclient.Interface,
) PolicyContextBuilder {
	return &policyContextBuilder{
		configuration: configuration,
		client:        client,
	}
}

func (b *policyContextBuilder) Build(request admissionv1.AdmissionRequest, roles, clusterRoles []string) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
		Roles:             roles,
		ClusterRoles:      clusterRoles,
	}
	return engine.NewPolicyContextFromAdmissionRequest(b.client.Discovery(), request, userRequestInfo, b.configuration)
}
