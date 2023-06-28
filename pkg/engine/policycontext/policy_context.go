package policycontext

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// operation contains the admission operatipn
	operation kyvernov1.AdmissionOperation

	// requestResource is GVR of the admission request
	requestResource metav1.GroupVersionResource

	// gvk is GVK of the top level resource
	gvk schema.GroupVersionKind

	// subresource is the subresource being requested, if any (for example, "status" or "scale")
	subresource string

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

func (c *PolicyContext) RequestResource() metav1.GroupVersionResource {
	return c.requestResource
}

func (c *PolicyContext) ResourceKind() (schema.GroupVersionKind, string) {
	// if the top level GVK is empty, fallback to the GVK of the resource
	if c.gvk.Empty() {
		if c.newResource.Object != nil {
			return c.newResource.GroupVersionKind(), ""
		} else {
			return c.oldResource.GroupVersionKind(), ""
		}
	}
	return c.gvk, c.subresource
}

func (c *PolicyContext) AdmissionInfo() kyvernov1beta1.RequestInfo {
	return c.admissionInfo
}

func (c *PolicyContext) Operation() kyvernov1.AdmissionOperation {
	return c.operation
}

func (c *PolicyContext) NamespaceLabels() map[string]string {
	return c.namespaceLabels
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

func (c *PolicyContext) WithResourceKind(gvk schema.GroupVersionKind, subresource string) *PolicyContext {
	copy := c.copy()
	copy.gvk = gvk
	copy.subresource = subresource
	return copy
}

func (c *PolicyContext) WithRequestResource(gvr metav1.GroupVersionResource) *PolicyContext {
	copy := c.copy()
	copy.requestResource = gvr
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

func (c PolicyContext) copy() *PolicyContext {
	return &c
}

// Constructors

func NewPolicyContextWithJsonContext(operation kyvernov1.AdmissionOperation, jsonContext enginectx.Interface) *PolicyContext {
	return &PolicyContext{
		operation:   operation,
		jsonContext: jsonContext,
	}
}

func NewPolicyContext(jp jmespath.Interface, operation kyvernov1.AdmissionOperation) *PolicyContext {
	return NewPolicyContextWithJsonContext(operation, enginectx.NewContext(jp))
}

func NewPolicyContextFromAdmissionRequest(
	jp jmespath.Interface,
	request admissionv1.AdmissionRequest,
	admissionInfo kyvernov1beta1.RequestInfo,
	gvk schema.GroupVersionKind,
	configuration config.Configuration,
) (*PolicyContext, error) {
	engineCtx, err := newJsonContext(jp, request, &admissionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy rule context: %w", err)
	}
	newResource, oldResource, err := admissionutils.ExtractResources(nil, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource: %w", err)
	}
	if err := engineCtx.AddImageInfos(&newResource, configuration); err != nil {
		return nil, fmt.Errorf("failed to add image information to the policy rule context: %w", err)
	}
	policyContext := NewPolicyContextWithJsonContext(kyvernov1.AdmissionOperation(request.Operation), engineCtx).
		WithNewResource(newResource).
		WithOldResource(oldResource).
		WithAdmissionInfo(admissionInfo).
		withAdmissionOperation(true).
		WithResourceKind(gvk, request.SubResource).
		WithRequestResource(request.Resource)
	return policyContext, nil
}

func newJsonContext(
	jp jmespath.Interface,
	request admissionv1.AdmissionRequest,
	userRequestInfo *kyvernov1beta1.RequestInfo,
) (enginectx.Interface, error) {
	engineCtx := enginectx.NewContext(jp)
	if err := engineCtx.AddRequest(request); err != nil {
		return nil, fmt.Errorf("failed to load incoming request in context: %w", err)
	}
	if err := engineCtx.AddUserInfo(*userRequestInfo); err != nil {
		return nil, fmt.Errorf("failed to load userInfo in context: %w", err)
	}
	if err := engineCtx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username); err != nil {
		return nil, fmt.Errorf("failed to load service account in context: %w", err)
	}
	return engineCtx, nil
}
