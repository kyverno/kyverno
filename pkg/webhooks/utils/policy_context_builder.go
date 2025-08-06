package utils

import (
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
	userRequestInfo := kyvernov2.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
		Roles:             roles,
		ClusterRoles:      clusterRoles,
	}

	if request.DryRun != nil {
		userRequestInfo.DryRun = *request.DryRun
	}
	return engine.NewPolicyContextFromAdmissionRequest(b.jp, request, userRequestInfo, gvk, b.configuration)
}
