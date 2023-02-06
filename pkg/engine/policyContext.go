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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// requestResource is the fully-qualified resource of the original API request (for example, v1.pods).
	// If this is specified and differs from the value in "resource", an equivalent match and conversion was performed.
	//
	// For example, if deployments can be modified via apps/v1 and apps/v1beta1, and a webhook registered a rule of
	// `apiGroups:["apps"], apiVersions:["v1"], resources: ["deployments"]` and `matchPolicy: Equivalent`,
	// an API request to apps/v1beta1 deployments would be converted and sent to the webhook
	// with `resource: {group:"apps", version:"v1", resource:"deployments"}` (matching the resource the webhook registered for),
	// and `requestResource: {group:"apps", version:"v1beta1", resource:"deployments"}` (indicating the resource of the original API request).
	requestResource metav1.GroupVersionResource

	// Dynamic client - used for api lookups
	client dclient.Interface

	// jsonContext is the variable context
	jsonContext enginectx.Interface

	// namespaceLabels stores the label of namespace to be processed by namespace selector
	namespaceLabels map[string]string

	// admissionOperation represents if the caller is from the webhook server
	admissionOperation bool

	// subresource is the subresource being requested, if any (for example, "status" or "scale")
	subresource string

	// subresourcesInPolicy represents the APIResources that are subresources along with their parent resource.
	// This is used to determine if a resource is a subresource. It is only used when the policy context is populated
	// by kyverno CLI. In all other cases when connected to a cluster, this is empty.
	subresourcesInPolicy []engineapi.SubResource
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

func (c *PolicyContext) SubResource() string {
	return c.subresource
}

func (c *PolicyContext) SubresourcesInPolicy() []engineapi.SubResource {
	return c.subresourcesInPolicy
}

func (c *PolicyContext) AdmissionOperation() bool {
	return c.admissionOperation
}

func (c *PolicyContext) RequestResource() metav1.GroupVersionResource {
	return c.requestResource
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

func (c *PolicyContext) Client() dclient.Interface {
	return c.client
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

func (c *PolicyContext) WithRequestResource(requestResource metav1.GroupVersionResource) *PolicyContext {
	copy := c.copy()
	copy.requestResource = requestResource
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

func (c *PolicyContext) WithClient(client dclient.Interface) *PolicyContext {
	copy := c.copy()
	copy.client = client
	return copy
}

func (c *PolicyContext) withAdmissionOperation(admissionOperation bool) *PolicyContext {
	copy := c.copy()
	copy.admissionOperation = admissionOperation
	return copy
}

func (c *PolicyContext) WithSubresource(subresource string) *PolicyContext {
	copy := c.copy()
	copy.subresource = subresource
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
	client dclient.Interface,
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
	requestResource := request.RequestResource.DeepCopy()
	policyContext := NewPolicyContextWithJsonContext(ctx).
		WithNewResource(newResource).
		WithOldResource(oldResource).
		WithAdmissionInfo(admissionInfo).
		WithClient(client).
		withAdmissionOperation(true).
		WithRequestResource(*requestResource).
		WithSubresource(request.SubResource)
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
