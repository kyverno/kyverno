package engine

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine/context"
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
	Config config.Interface
}
