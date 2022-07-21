package client

import (
	"fmt"
	"strings"
	"time"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"

	openapiv2 "github.com/googleapis/gnostic/openapiv2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

// IDiscovery provides interface to mange Kind and GVR mapping
type IDiscovery interface {
	FindResource(apiVersion string, kind string) (*metav1.APIResource, schema.GroupVersionResource, error)
	GetGVRFromKind(kind string) (schema.GroupVersionResource, error)
	GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource
	GetServerVersion() (*version.Info, error)
	OpenAPISchema() (*openapiv2.Document, error)
	DiscoveryCache() discovery.CachedDiscoveryInterface
	DiscoveryInterface() discovery.DiscoveryInterface
}

// serverPreferredResources stores the cachedClient instance for discovery client
type serverPreferredResources struct {
	cachedClient discovery.CachedDiscoveryInterface
}

// DiscoveryCache gets the discovery client cache
func (c serverPreferredResources) DiscoveryCache() discovery.CachedDiscoveryInterface {
	return c.cachedClient
}

// DiscoveryInterface gets the discovery client
func (c serverPreferredResources) DiscoveryInterface() discovery.DiscoveryInterface {
	return c.cachedClient
}

// Poll will keep invalidate the local cache
func (c serverPreferredResources) Poll(resync time.Duration, stopCh <-chan struct{}) {
	logger := logger.WithName("Poll")
	// start a ticker
	ticker := time.NewTicker(resync)
	defer func() { ticker.Stop() }()
	logger.V(4).Info("starting registered resources sync", "period", resync)
	for {
		select {
		case <-stopCh:
			logger.Info("stopping registered resources sync")
			return
		case <-ticker.C:
			// set cache as stale
			logger.V(6).Info("invalidating local client cache for registered resources")
			c.cachedClient.Invalidate()
		}
	}
}

// OpenAPISchema returns the API server OpenAPI schema document
func (c serverPreferredResources) OpenAPISchema() (*openapiv2.Document, error) {
	return c.cachedClient.OpenAPISchema()
}

// GetGVRFromKind get the Group Version Resource from kind
func (c serverPreferredResources) GetGVRFromKind(kind string) (schema.GroupVersionResource, error) {
	if kind == "" {
		return schema.GroupVersionResource{}, nil
	}
	_, k := kubeutils.GetKindFromGVK(kind)
	_, gvr, err := c.FindResource("", k)
	if err != nil {
		logger.Info("schema not found", "kind", k)
		return schema.GroupVersionResource{}, err
	}

	return gvr, nil
}

// GetGVRFromAPIVersionKind get the Group Version Resource from APIVersion and kind
func (c serverPreferredResources) GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource {
	_, gvr, err := c.FindResource(apiVersion, kind)
	if err != nil {
		logger.Info("schema not found", "kind", kind, "apiVersion", apiVersion, "error : ", err)
		return schema.GroupVersionResource{}
	}

	return gvr
}

// GetServerVersion returns the server version of the cluster
func (c serverPreferredResources) GetServerVersion() (*version.Info, error) {
	return c.cachedClient.ServerVersion()
}

// FindResource finds an API resource that matches 'kind'. If the resource is not
// found and the Cache is not fresh, the cache is invalidated and a retry is attempted
func (c serverPreferredResources) FindResource(apiVersion string, kind string) (*metav1.APIResource, schema.GroupVersionResource, error) {
	r, gvr, err := c.findResource(apiVersion, kind)
	if err == nil {
		return r, gvr, nil
	}

	if !c.cachedClient.Fresh() {
		c.cachedClient.Invalidate()
		if r, gvr, err = c.findResource(apiVersion, kind); err == nil {
			return r, gvr, nil
		}
	}

	return nil, schema.GroupVersionResource{}, err
}

func (c serverPreferredResources) findResource(apiVersion string, kind string) (*metav1.APIResource, schema.GroupVersionResource, error) {
	var serverResources []*metav1.APIResourceList
	var err error
	if apiVersion == "" {
		serverResources, err = c.cachedClient.ServerPreferredResources()
	} else {
		_, serverResources, err = c.cachedClient.ServerGroupsAndResources()
	}

	if err != nil && !strings.Contains(err.Error(), "Got empty response for") {
		if discovery.IsGroupDiscoveryFailedError(err) {
			logDiscoveryErrors(err, c)
		} else if isMetricsServerUnavailable(kind, err) {
			logger.V(3).Info("failed to find preferred resource version", "error", err.Error())
		} else {
			logger.Error(err, "failed to find preferred resource version")
			return nil, schema.GroupVersionResource{}, err
		}
	}

	k, subresource := kubeutils.SplitSubresource(kind)
	if subresource != "" {
		kind = k
	}

	for _, serverResource := range serverResources {
		if apiVersion != "" && serverResource.GroupVersion != apiVersion {
			continue
		}

		for _, resource := range serverResource.APIResources {
			if resourceMatches(resource, kind, subresource) {
				logger.V(4).Info("matched API resource to kind", "apiResource", resource, "kind", kind)
				gv, err := schema.ParseGroupVersion(serverResource.GroupVersion)
				if err != nil {
					logger.Error(err, "failed to parse GV", "groupVersion", serverResource.GroupVersion)
					return nil, schema.GroupVersionResource{}, err
				}

				return &resource, gv.WithResource(resource.Name), nil
			}
		}
	}

	return nil, schema.GroupVersionResource{}, fmt.Errorf("kind '%s' not found in apiVersion '%s'", kind, apiVersion)
}

// resourceMatches checks the resource Kind, Name, SingularName and a subresource if specified
// e.g. &apiResource{Name: "taskruns/status", Kind: "TaskRun"} will match "kind=TaskRun, subresource=Status"
func resourceMatches(resource metav1.APIResource, kind, subresource string) bool {
	if resource.Kind == kind || resource.Name == kind || resource.SingularName == kind {
		_, s := kubeutils.SplitSubresource(resource.Name)
		return strings.EqualFold(s, subresource)
	}

	return false
}
