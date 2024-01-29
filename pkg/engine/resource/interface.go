package resource

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

// TODO: Placeholder
type ContextEntry struct {
	// Name is the variable name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// ConfigMap is the ConfigMap reference.
	ConfigMap *kyvernov1.ConfigMapReference `json:"configMap,omitempty" yaml:"configMap,omitempty"`

	// APICall is an HTTP request to the Kubernetes API server, or other JSON web service.
	// The data returned is stored in the context with the name for the context entry.
	APICall *kyvernov1.APICall `json:"apiCall,omitempty" yaml:"apiCall,omitempty"`

	// ImageRegistry defines requests to an OCI/Docker V2 registry to fetch image
	// details.
	ImageRegistry *kyvernov1.ImageRegistry `json:"imageRegistry,omitempty" yaml:"imageRegistry,omitempty"`

	// Variable defines an arbitrary JMESPath context variable that can be defined inline.
	Variable *kyvernov1.Variable `json:"variable,omitempty" yaml:"variable,omitempty"`

	// ResourceCache is the request to the cache to fetch a specific cache entry.
	Resource *kyvernov1.ResourceCache `json:"resource,omitempty" yaml:"resource,omitempty"`
}

type Interface interface {
	Get(ContextEntry, enginecontext.Interface) ([]byte, error)
}
