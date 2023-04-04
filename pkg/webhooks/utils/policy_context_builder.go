package utils

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyContextBuilder interface {
	Build(admissionv1.AdmissionRequest, []string, []string, schema.GroupVersionKind) (*engine.PolicyContext, error)
}

type policyContextBuilder struct {
	configuration config.Configuration
}

func NewPolicyContextBuilder(
	configuration config.Configuration,
) PolicyContextBuilder {
	return &policyContextBuilder{
		configuration: configuration,
	}
}

func (b *policyContextBuilder) Build(request admissionv1.AdmissionRequest, roles, clusterRoles []string, gvk schema.GroupVersionKind) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
		Roles:             roles,
		ClusterRoles:      clusterRoles,
	}
	return engine.NewPolicyContextFromAdmissionRequest(request, userRequestInfo, gvk, b.configuration)
}
