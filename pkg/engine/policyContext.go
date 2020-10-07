package engine

import (
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext struct {
	// policy to be processed
	Policy kyverno.ClusterPolicy
	// resource to be processed
	NewResource unstructured.Unstructured
	// old Resource - Update operations
	OldResource   unstructured.Unstructured
	AdmissionInfo kyverno.RequestInfo
	// Dynamic client - used by generate
	Client *client.Client
	// Contexts to store resources
	Context context.EvalInterface
	// Config handler
	ExcludeGroupRole []string

	// ResourceCache provides listers to resources
	// Currently Supports Configmap
	ResourceCache resourcecache.ResourceCacheIface
	// JSONContext ...
	JSONContext *context.Context
}
