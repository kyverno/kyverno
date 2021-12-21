package engine

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext struct {

	// Policy is the policy to be processed
	Policy kyverno.ClusterPolicy

	// NewResource is the resource to be processed
	NewResource unstructured.Unstructured

	// OldResource is the prior resource for an update, or nil
	OldResource unstructured.Unstructured

	// Element is set when the context is used for processing a foreach loop
	Element unstructured.Unstructured

	// AdmissionInfo contains the admission request information
	AdmissionInfo kyverno.RequestInfo

	// Dynamic client - used by generate
	Client *client.Client

	// Config handler
	ExcludeGroupRole []string

	ExcludeResourceFunc func(kind, namespace, name string) bool

	// ResourceCache provides listers to resources. Currently Supports Configmap
	ResourceCache resourcecache.ResourceCache

	// JSONContext is the variable context
	JSONContext *context.Context

	// NamespaceLabels stores the label of namespace to be processed by namespace selector
	NamespaceLabels map[string]string
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
		ResourceCache:       pc.ResourceCache,
		JSONContext:         pc.JSONContext,
		NamespaceLabels:     pc.NamespaceLabels,
	}
}
