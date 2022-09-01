package utils

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type PolicyContextBuilder interface {
	Build(*admissionv1.AdmissionRequest, bool) (*engine.PolicyContext, error)
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

func convertResource(request *admissionv1.AdmissionRequest, resourceRaw []byte) (unstructured.Unstructured, error) {
	resource, err := utils.ConvertResource(resourceRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert raw resource to unstructured format")
	}
	if request.Kind.Kind == "Secret" && request.Operation == admissionv1.Update {
		resource, err = utils.NormalizeSecret(&resource)
		if err != nil {
			return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert secret to unstructured format")
		}
	}
	return resource, nil
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

func (b *policyContextBuilder) Build(request *admissionv1.AdmissionRequest, addRoles bool) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}
	if addRoles {
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
	resource, err := convertResource(request, request.Object.Raw)
	if err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}
	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    b.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: b.configuration.ToFilter,
		JSONContext:         ctx,
		Client:              b.client,
		AdmissionOperation:  true,
	}
	if request.Operation == admissionv1.Update {
		policyContext.OldResource, err = convertResource(request, request.OldObject.Raw)
		if err != nil {
			return nil, err
		}
	}
	return policyContext, nil
}
