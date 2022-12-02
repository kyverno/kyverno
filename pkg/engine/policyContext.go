package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ExcludeFunc is a function used to determine if a resource is excluded
type ExcludeFunc = func(kind, namespace, name string) bool

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

	// Dynamic client - used for api lookups
	client dclient.Interface

	// Config handler
	excludeGroupRole []string

	excludeResourceFunc ExcludeFunc

	// jsonContext is the variable context
	jsonContext context.Interface

	// namespaceLabels stores the label of namespace to be processed by namespace selector
	namespaceLabels map[string]string

	// admissionOperation represents if the caller is from the webhook server
	admissionOperation bool

	// informerCacheResolvers - used to get resources from informer cache
	informerCacheResolvers resolvers.ConfigmapResolver
}

// Getters

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

func (c *PolicyContext) JSONContext() context.Interface {
	return c.jsonContext
}

// Mutators

func (c *PolicyContext) WithPolicy(policy kyvernov1.PolicyInterface) *PolicyContext {
	copy := c.Copy()
	copy.policy = policy
	return copy
}

func (c *PolicyContext) WithNamespaceLabels(namespaceLabels map[string]string) *PolicyContext {
	copy := c.Copy()
	copy.namespaceLabels = namespaceLabels
	return copy
}

func (c *PolicyContext) WithAdmissionInfo(admissionInfo kyvernov1beta1.RequestInfo) *PolicyContext {
	copy := c.Copy()
	copy.admissionInfo = admissionInfo
	return copy
}

func (c *PolicyContext) WithNewResource(resource unstructured.Unstructured) *PolicyContext {
	copy := c.Copy()
	copy.newResource = resource
	return copy
}

func (c *PolicyContext) WithOldResource(resource unstructured.Unstructured) *PolicyContext {
	copy := c.Copy()
	copy.oldResource = resource
	return copy
}

func (c *PolicyContext) WithResources(newResource unstructured.Unstructured, oldResource unstructured.Unstructured) *PolicyContext {
	return c.WithNewResource(newResource).WithOldResource(oldResource)
}

func (c *PolicyContext) WithClient(client dclient.Interface) *PolicyContext {
	copy := c.Copy()
	copy.client = client
	return copy
}

func (c *PolicyContext) WithExcludeGroupRole(excludeGroupRole ...string) *PolicyContext {
	copy := c.Copy()
	copy.excludeGroupRole = excludeGroupRole
	return copy
}

func (c *PolicyContext) WithExcludeResourceFunc(excludeResourceFunc ExcludeFunc) *PolicyContext {
	copy := c.Copy()
	copy.excludeResourceFunc = excludeResourceFunc
	return copy
}

func (c *PolicyContext) WithConfiguration(configuration config.Configuration) *PolicyContext {
	return c.WithExcludeResourceFunc(configuration.ToFilter).WithExcludeGroupRole(configuration.GetExcludeGroupRole()...)
}

func (c *PolicyContext) WithAdmissionOperation(admissionOperation bool) *PolicyContext {
	copy := c.Copy()
	copy.admissionOperation = admissionOperation
	return copy
}

func (c *PolicyContext) WithInformerCacheResolver(informerCacheResolver resolvers.ConfigmapResolver) *PolicyContext {
	copy := c.Copy()
	copy.informerCacheResolvers = informerCacheResolver
	return copy
}

// Constructors

func NewPolicyContextWithJsonContext(jsonContext context.Interface) *PolicyContext {
	return &PolicyContext{
		jsonContext:      jsonContext,
		excludeGroupRole: []string{},
		excludeResourceFunc: func(string, string, string) bool {
			return false
		},
	}
}

func NewPolicyContext() *PolicyContext {
	return NewPolicyContextWithJsonContext(context.NewContext())
}

func NewPolicyContextFromAdmissionRequest(
	request *admissionv1.AdmissionRequest,
	admissionInfo kyvernov1beta1.RequestInfo,
	configuration config.Configuration,
	client dclient.Interface,
	informerCacheResolver resolvers.ConfigmapResolver,
) (*PolicyContext, error) {
	ctx, err := newVariablesContext(request, &admissionInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy rule context")
	}
	newResource, oldResource, err := utils.ExtractResources(nil, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource")
	}
	if err := ctx.AddImageInfos(&newResource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}
	policyContext := NewPolicyContextWithJsonContext(ctx).
		WithNewResource(newResource).
		WithOldResource(oldResource).
		WithAdmissionInfo(admissionInfo).
		WithConfiguration(configuration).
		WithClient(client).
		WithAdmissionOperation(true).
		WithInformerCacheResolver(informerCacheResolver)
	return policyContext, nil
}

func (c PolicyContext) Copy() *PolicyContext {
	return &c
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
