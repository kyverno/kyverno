package utils

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PolicyContextBuilder interface {
	Build(admissionv1.AdmissionRequest, []string, []string, schema.GroupVersionKind) (*engine.PolicyContext, error)
}

type policyContextBuilder struct {
	configuration config.Configuration
	jp            jmespath.Interface
}

func NewPolicyContextBuilder(
	configuration config.Configuration,
	jp jmespath.Interface,
) PolicyContextBuilder {
	return &policyContextBuilder{
		configuration: configuration,
		jp:            jp,
	}
}

func (b *policyContextBuilder) Build(request admissionv1.AdmissionRequest, roles, clusterRoles []string, gvk schema.GroupVersionKind) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
		Roles:             roles,
		ClusterRoles:      clusterRoles,
	}
	return engine.NewPolicyContextFromAdmissionRequest(b.jp, request, userRequestInfo, gvk, b.configuration)
}
