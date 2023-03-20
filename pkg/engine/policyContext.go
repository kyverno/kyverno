package engine

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext struct {
	// policy is the policy to be processed
	policy kyvernov1.PolicyInterface

	// newResource is the resource to be processed
	newResource unstructured.Unstructured

	// oldResource is the prior resource for an update, or nil
	oldResource unstructured.Unstructured

	// element is set when the context is used for processing a foreach loop
	element unstructured.Unstructured

	// admissionInfo contains the admission request information
	admissionInfo kyvernov1beta1.RequestInfo

	// subresourcesInPolicy represents the APIResources that are subresources along with their parent resource.
	// This is used to determine if a resource is a subresource. It is only used when the policy context is populated
	// by kyverno CLI. In all other cases when connected to a cluster, this is empty.
	subresourcesInPolicy []engineapi.SubResource

	gvrs dclient.GroupVersionResourceSubresource

	// jsonContext is the variable context
	jsonContext enginectx.Interface

	// namespaceLabels stores the label of namespace to be processed by namespace selector
	namespaceLabels map[string]string

	// admissionOperation represents if the caller is from the webhook server
	admissionOperation bool
}

// engineapi.PolicyContext interface

func (c *PolicyContext) Policy() kyvernov1.PolicyInterface {
	return c.policy
}

func (c *PolicyContext) NewResource() unstructured.Unstructured {
	return c.newResource
}

func (c *PolicyContext) OldResource() unstructured.Unstructured {
	return c.oldResource
}

func (c *PolicyContext) AdmissionInfo() kyvernov1beta1.RequestInfo {
	return c.admissionInfo
}

func (c *PolicyContext) NamespaceLabels() map[string]string {
	return c.namespaceLabels
}

func (c *PolicyContext) GroupVersionResourceSubresource() dclient.GroupVersionResourceSubresource {
	return c.gvrs
}

func (c *PolicyContext) SubresourcesInPolicy() []engineapi.SubResource {
	return c.subresourcesInPolicy
}

func (c *PolicyContext) AdmissionOperation() bool {
	return c.admissionOperation
}

func (c *PolicyContext) Element() unstructured.Unstructured {
	return c.element
}

func (c *PolicyContext) SetElement(element unstructured.Unstructured) {
	c.element = element
}

func (c *PolicyContext) JSONContext() enginectx.Interface {
	return c.jsonContext
}

func (c PolicyContext) Copy() engineapi.PolicyContext {
	return c.copy()
}

// Mutators

func (c *PolicyContext) WithPolicy(policy kyvernov1.PolicyInterface) *PolicyContext {
	copy := c.copy()
	copy.policy = policy
	return copy
}

func (c *PolicyContext) WithNamespaceLabels(namespaceLabels map[string]string) *PolicyContext {
	copy := c.copy()
	copy.namespaceLabels = namespaceLabels
	return copy
}

func (c *PolicyContext) WithAdmissionInfo(admissionInfo kyvernov1beta1.RequestInfo) *PolicyContext {
	copy := c.copy()
	copy.admissionInfo = admissionInfo
	return copy
}

func (c *PolicyContext) WithNewResource(resource unstructured.Unstructured) *PolicyContext {
	copy := c.copy()
	copy.newResource = resource
	return copy
}

func (c *PolicyContext) WithOldResource(resource unstructured.Unstructured) *PolicyContext {
	copy := c.copy()
	copy.oldResource = resource
	return copy
}

func (c *PolicyContext) WithResources(newResource unstructured.Unstructured, oldResource unstructured.Unstructured) *PolicyContext {
	return c.WithNewResource(newResource).WithOldResource(oldResource)
}

func (c *PolicyContext) withAdmissionOperation(admissionOperation bool) *PolicyContext {
	copy := c.copy()
	copy.admissionOperation = admissionOperation
	return copy
}

func (c *PolicyContext) WithGroupVersionResourceSubresource(gvrs dclient.GroupVersionResourceSubresource) *PolicyContext {
	copy := c.copy()
	copy.gvrs = gvrs
	return copy
}

func (c *PolicyContext) WithSubresourcesInPolicy(subresourcesInPolicy []engineapi.SubResource) *PolicyContext {
	copy := c.copy()
	copy.subresourcesInPolicy = subresourcesInPolicy
	return copy
}

func (c PolicyContext) copy() *PolicyContext {
	return &c
}

// Constructors

func NewPolicyContextWithJsonContext(jsonContext enginectx.Interface) *PolicyContext {
	return &PolicyContext{
		jsonContext: jsonContext,
	}
}

func NewPolicyContext() *PolicyContext {
	return NewPolicyContextWithJsonContext(enginectx.NewContext())
}

func NewPolicyContextFromAdmissionRequest(
	request *admissionv1.AdmissionRequest,
	admissionInfo kyvernov1beta1.RequestInfo,
	configuration config.Configuration,
) (*PolicyContext, error) {
	ctx, err := newVariablesContext(request, &admissionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy rule context: %w", err)
	}
	newResource, oldResource, err := admissionutils.ExtractResources(nil, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource: %w", err)
	}
	if err := ctx.AddImageInfos(&newResource, configuration); err != nil {
		return nil, fmt.Errorf("failed to add image information to the policy rule context: %w", err)
	}
	// requestResource := request.RequestResource.DeepCopy()
	policyContext := NewPolicyContextWithJsonContext(ctx).
		WithNewResource(newResource).
		WithOldResource(oldResource).
		WithAdmissionInfo(admissionInfo).
		withAdmissionOperation(true).
		WithGroupVersionResourceSubresource(dclient.GroupVersionResourceSubresource{
			GroupVersionResource: schema.GroupVersionResource(request.Resource),
			SubResource:          request.SubResource,
		})
	return policyContext, nil
}

func newVariablesContext(request *admissionv1.AdmissionRequest, userRequestInfo *kyvernov1beta1.RequestInfo) (enginectx.Interface, error) {
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(request); err != nil {
		return nil, fmt.Errorf("failed to load incoming request in context: %w", err)
	}
	if err := ctx.AddUserInfo(*userRequestInfo); err != nil {
		return nil, fmt.Errorf("failed to load userInfo in context: %w", err)
	}
	if err := ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username); err != nil {
		return nil, fmt.Errorf("failed to load service account in context: %w", err)
	}
	return ctx, nil
}
