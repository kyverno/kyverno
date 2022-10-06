package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext struct {
	// Policy is the policy to be processed
	Policy kyvernov1.PolicyInterface

	// NewResource is the resource to be processed
	NewResource unstructured.Unstructured

	// OldResource is the prior resource for an update, or nil
	OldResource unstructured.Unstructured

	// Element is set when the context is used for processing a foreach loop
	Element unstructured.Unstructured

	// AdmissionInfo contains the admission request information
	AdmissionInfo kyvernov1beta1.RequestInfo

	// Dynamic client - used for api lookups
	Client dclient.Interface

	// Config handler
	ExcludeGroupRole []string

	ExcludeResourceFunc func(kind, namespace, name string) bool

	// JSONContext is the variable context
	JSONContext context.Interface

	// NamespaceLabels stores the label of namespace to be processed by namespace selector
	NamespaceLabels map[string]string

	// AdmissionOperation represents if the caller is from the webhook server
	AdmissionOperation bool
}

func (pc *PolicyContext) Copy() *PolicyContext {
	return &PolicyContext{
		Policy:              pc.Policy,
		NewResource:         pc.NewResource,
		OldResource:         pc.OldResource,
		AdmissionInfo:       pc.AdmissionInfo,
		Client:              pc.Client,
		ExcludeGroupRole:    pc.ExcludeGroupRole,
		ExcludeResourceFunc: pc.ExcludeResourceFunc,
		JSONContext:         pc.JSONContext,
		NamespaceLabels:     pc.NamespaceLabels,
	}
}
