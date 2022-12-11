package dclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	openapiv2 "github.com/google/gnostic/openapiv2"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

// IDiscovery provides interface to mange Kind and GVR mapping
type IDiscovery interface {
	FindResource(groupVersion string, kind string) (apiResource, parentAPIResource *metav1.APIResource, gvr schema.GroupVersionResource, err error)
	GetGVRFromKind(kind string) (schema.GroupVersionResource, error)
	GetGVRFromAPIVersionKind(groupVersion string, kind string) schema.GroupVersionResource
	GetGVKFromGVR(apiVersion, resourceName string) (schema.GroupVersionKind, error)
	GetServerVersion() (*version.Info, error)
	OpenAPISchema() (*openapiv2.Document, error)
	DiscoveryCache() discovery.CachedDiscoveryInterface
	DiscoveryInterface() discovery.DiscoveryInterface
}

// apiResourceWithListGV is a wrapper for metav1.APIResource with the group-version of its metav1.APIResourceList
type apiResourceWithListGV struct {
	apiResource metav1.APIResource
	listGV      string
}

// serverResources stores the cachedClient instance for discovery client
type serverResources struct {
	cachedClient discovery.CachedDiscoveryInterface
}

// DiscoveryCache gets the discovery client cache
func (c serverResources) DiscoveryCache() discovery.CachedDiscoveryInterface {
	return c.cachedClient
}

// DiscoveryInterface gets the discovery client
func (c serverResources) DiscoveryInterface() discovery.DiscoveryInterface {
	return c.cachedClient
}

// Poll will keep invalidate the local cache
func (c serverResources) Poll(ctx context.Context, resync time.Duration) {
	logger := logger.WithName("Poll")
	// start a ticker
	ticker := time.NewTicker(resync)
	defer func() { ticker.Stop() }()
	logger.V(6).Info("starting registered resources sync", "period", resync)
	for {
		select {
		case <-ctx.Done():
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
func (c serverResources) OpenAPISchema() (*openapiv2.Document, error) {
	return c.cachedClient.OpenAPISchema()
}

// GetGVRFromKind get the Group Version Resource from kind
func (c serverResources) GetGVRFromKind(kind string) (schema.GroupVersionResource, error) {
	if kind == "" {
		return schema.GroupVersionResource{}, nil
	}
	gv, k := kubeutils.GetKindFromGVK(kind)
	_, _, gvr, err := c.FindResource(gv, k)
	if err != nil {
		logger.Info("schema not found", "kind", k)
		return schema.GroupVersionResource{}, err
	}

	return gvr, nil
}

// GetGVRFromAPIVersionKind get the Group Version Resource from APIVersion and kind
func (c serverResources) GetGVRFromAPIVersionKind(apiVersion string, kind string) schema.GroupVersionResource {
	_, _, gvr, err := c.FindResource(apiVersion, kind)
	if err != nil {
		logger.Info("schema not found", "kind", kind, "apiVersion", apiVersion, "error : ", err)
		return schema.GroupVersionResource{}
	}

	return gvr
}

// GetServerVersion returns the server version of the cluster
func (c serverResources) GetServerVersion() (*version.Info, error) {
	return c.cachedClient.ServerVersion()
}

// GetGVKFromGVR returns the Group Version Kind from Group Version Resource. The groupVersion has to be specified properly
// for example, for corev1.Pod, the groupVersion has to be specified as `v1`, specifying empty groupVersion won't work.
func (c serverResources) GetGVKFromGVR(groupVersion, resourceName string) (schema.GroupVersionKind, error) {
	gvk, err := c.findResourceFromResourceName(groupVersion, resourceName)
	if err == nil {
		return gvk, nil
	}

	if !c.cachedClient.Fresh() {
		c.cachedClient.Invalidate()
		if gvk, err := c.findResourceFromResourceName(groupVersion, resourceName); err == nil {
			return gvk, nil
		}
	}

	return schema.GroupVersionKind{}, err
}

// findResourceFromResourceName returns the GVK for the a particular resourceName and groupVersion
func (c serverResources) findResourceFromResourceName(groupVersion, resourceName string) (schema.GroupVersionKind, error) {
	_, serverGroupsAndResources, err := c.cachedClient.ServerGroupsAndResources()
	if err != nil && !strings.Contains(err.Error(), "Got empty response for") {
		if discovery.IsGroupDiscoveryFailedError(err) {
			logDiscoveryErrors(err)
		} else if isMetricsServerUnavailable(groupVersion, err) {
			logger.V(3).Info("failed to find preferred resource version", "error", err.Error())
		} else {
			logger.Error(err, "failed to find preferred resource version")
			return schema.GroupVersionKind{}, err
		}
	}
	apiResource, err := findResourceFromResourceName(groupVersion, resourceName, serverGroupsAndResources)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return schema.GroupVersionKind{Group: apiResource.Group, Version: apiResource.Version, Kind: apiResource.Kind}, err
}

// FindResource finds an API resource that matches 'kind'. For finding subresources that have the same kind as the parent
// resource, kind has to be specified as 'ParentKind/SubresourceName'. For matching status subresource of Pod, kind has
// to be specified as `Pod/status`. If the resource is not found and the Cache is not fresh, the cache is invalidated
// and a retry is attempted
func (c serverResources) FindResource(groupVersion string, kind string) (apiResource, parentAPIResource *metav1.APIResource, gvr schema.GroupVersionResource, err error) {
	r, pr, gvr, err := c.findResource(groupVersion, kind)
	if err == nil {
		return r, pr, gvr, nil
	}

	if !c.cachedClient.Fresh() {
		c.cachedClient.Invalidate()
		if r, pr, gvr, err = c.findResource(groupVersion, kind); err == nil {
			return r, pr, gvr, nil
		}
	}

	return nil, nil, schema.GroupVersionResource{}, err
}

func (c serverResources) findResource(groupVersion string, kind string) (apiResource, parentAPIResource *metav1.APIResource,
	gvr schema.GroupVersionResource, err error,
) {
	serverPreferredResources, _ := c.cachedClient.ServerPreferredResources()
	_, serverGroupsAndResources, err := c.cachedClient.ServerGroupsAndResources()

	if err != nil && !strings.Contains(err.Error(), "Got empty response for") {
		if discovery.IsGroupDiscoveryFailedError(err) {
			logDiscoveryErrors(err)
		} else if isMetricsServerUnavailable(groupVersion, err) {
			logger.V(3).Info("failed to find preferred resource version", "error", err.Error())
		} else {
			logger.Error(err, "failed to find preferred resource version")
			return nil, nil, schema.GroupVersionResource{}, err
		}
	}

	kindWithoutSubresource, subresource := kubeutils.SplitSubresource(kind)

	if subresource != "" {
		parentApiResource, _, _, err := c.findResource(groupVersion, kindWithoutSubresource)
		if err != nil {
			logger.Error(err, "Unable to find parent resource", "kind", kind)
			return nil, nil, schema.GroupVersionResource{}, err
		}
		parentResourceName := parentApiResource.Name
		resource, gvr, err := findSubresource(groupVersion, parentResourceName, subresource, kind, serverGroupsAndResources)
		return resource, parentApiResource, gvr, err
	}

	return findResource(groupVersion, kind, serverPreferredResources, serverGroupsAndResources)
}

// findSubresource finds the subresource for the given parent resource, groupVersion and serverResourcesList
func findSubresource(groupVersion, parentResourceName, subresource, kind string, serverResourcesList []*metav1.APIResourceList) (
	apiResource *metav1.APIResource, gvr schema.GroupVersionResource, err error,
) {
	for _, serverResourceList := range serverResourcesList {
		if groupVersion == "" || kubeutils.GroupVersionMatches(groupVersion, serverResourceList.GroupVersion) {
			for _, serverResource := range serverResourceList.APIResources {
				if serverResource.Name == parentResourceName+"/"+strings.ToLower(subresource) {
					logger.V(6).Info("matched API resource to kind", "apiResource", serverResource, "kind", kind)

					serverResourceGv := getServerResourceGroupVersion(serverResourceList.GroupVersion, serverResource.Group, serverResource.Version)
					gv, _ := schema.ParseGroupVersion(serverResourceGv)

					serverResource.Group = gv.Group
					serverResource.Version = gv.Version

					groupVersionResource := gv.WithResource(serverResource.Name)
					logger.V(6).Info("gv with resource", "gvWithResource", groupVersionResource)
					return &serverResource, groupVersionResource, nil
				}
			}
		}
	}

	return nil, schema.GroupVersionResource{}, fmt.Errorf("resource not found for kind %s", kind)
}

// findResource finds an API resource that matches 'groupVersion', 'kind', in the given serverResourcesList
func findResource(groupVersion string, kind string, serverPreferredResources, serverGroupsAndResources []*metav1.APIResourceList) (
	apiResource, parentAPIResource *metav1.APIResource, gvr schema.GroupVersionResource, err error,
) {
	matchingServerResources := getMatchingServerResources(groupVersion, kind, serverGroupsAndResources)

	onlySubresourcePresentInMatchingResources := len(matchingServerResources) > 0
	for _, matchingServerResource := range matchingServerResources {
		if !kubeutils.IsSubresource(matchingServerResource.apiResource.Name) {
			onlySubresourcePresentInMatchingResources = false
			break
		}
	}

	if onlySubresourcePresentInMatchingResources {
		apiResourceWithListGV := matchingServerResources[0]
		matchingServerResource := apiResourceWithListGV.apiResource
		logger.V(6).Info("matched API resource to kind", "apiResource", matchingServerResource, "kind", kind)

		groupVersionResource := schema.GroupVersionResource{
			Resource: matchingServerResource.Name,
			Group:    matchingServerResource.Group,
			Version:  matchingServerResource.Version,
		}
		logger.V(6).Info("gv with resource", "gvWithResource", groupVersionResource)

		parentAPIResource, err := findResourceFromResourceName(apiResourceWithListGV.listGV, strings.Split(matchingServerResource.Name, "/")[0], serverPreferredResources)
		if err != nil {
			return nil, nil, schema.GroupVersionResource{}, fmt.Errorf("failed to find parent resource for subresource %s: %v", matchingServerResource.Name, err)
		}
		logger.V(6).Info("parent API resource", "parentAPIResource", parentAPIResource)

		return &matchingServerResource, parentAPIResource, groupVersionResource, nil
	}

	if groupVersion == "" && len(matchingServerResources) > 0 {
		for _, serverResourceList := range serverPreferredResources {
			for _, serverResource := range serverResourceList.APIResources {
				serverResourceGv := getServerResourceGroupVersion(serverResourceList.GroupVersion, serverResource.Group, serverResource.Version)
				if serverResource.Kind == kind || serverResource.SingularName == kind {
					gv, _ := schema.ParseGroupVersion(serverResourceGv)
					serverResource.Group = gv.Group
					serverResource.Version = gv.Version
					groupVersionResource := gv.WithResource(serverResource.Name)

					logger.V(6).Info("matched API resource to kind", "apiResource", serverResource, "kind", kind)
					return &serverResource, nil, groupVersionResource, nil
				}
			}
		}
	} else {
		for _, apiResourceWithListGV := range matchingServerResources {
			matchingServerResource := apiResourceWithListGV.apiResource
			if !kubeutils.IsSubresource(matchingServerResource.Name) {
				logger.V(6).Info("matched API resource to kind", "apiResource", matchingServerResource, "kind", kind)

				groupVersionResource := schema.GroupVersionResource{
					Resource: matchingServerResource.Name,
					Group:    matchingServerResource.Group,
					Version:  matchingServerResource.Version,
				}
				logger.V(6).Info("gv with resource", "groupVersionResource", groupVersionResource)
				return &matchingServerResource, nil, groupVersionResource, nil
			}
		}
	}

	return nil, nil, schema.GroupVersionResource{}, fmt.Errorf("kind '%s' not found in groupVersion '%s'", kind, groupVersion)
}

// getMatchingServerResources returns a list of API resources that match the given groupVersion and kind
func getMatchingServerResources(groupVersion string, kind string, serverGroupsAndResources []*metav1.APIResourceList) []apiResourceWithListGV {
	matchingServerResources := make([]apiResourceWithListGV, 0)
	for _, serverResourceList := range serverGroupsAndResources {
		for _, serverResource := range serverResourceList.APIResources {
			serverResourceGv := getServerResourceGroupVersion(serverResourceList.GroupVersion, serverResource.Group, serverResource.Version)
			if groupVersion == "" || kubeutils.GroupVersionMatches(groupVersion, serverResourceGv) {
				if serverResource.Kind == kind || serverResource.SingularName == kind {
					gv, _ := schema.ParseGroupVersion(serverResourceGv)
					serverResource.Group = gv.Group
					serverResource.Version = gv.Version
					matchingServerResources = append(matchingServerResources, apiResourceWithListGV{apiResource: serverResource, listGV: serverResourceList.GroupVersion})
				}
			}
		}
	}
	return matchingServerResources
}

// findResourceFromResourceName finds an API resource that matches 'resourceName', in the given serverResourcesList
func findResourceFromResourceName(groupVersion string, resourceName string, serverGroupsAndResources []*metav1.APIResourceList) (*metav1.APIResource, error) {
	for _, serverResourceList := range serverGroupsAndResources {
		for _, apiResource := range serverResourceList.APIResources {
			serverResourceGroupVersion := getServerResourceGroupVersion(serverResourceList.GroupVersion, apiResource.Group, apiResource.Version)
			if serverResourceGroupVersion == groupVersion && apiResource.Name == resourceName {
				logger.V(6).Info("found preferred resource", "groupVersion", groupVersion, "resourceName", resourceName)
				groupVersion, _ := schema.ParseGroupVersion(serverResourceGroupVersion)
				apiResource.Group = groupVersion.Group
				apiResource.Version = groupVersion.Version
				return &apiResource, nil
			}
		}
	}
	return nil, fmt.Errorf("resource %s not found in group %s", resourceName, groupVersion)
}

// getServerResourceGroupVersion returns the groupVersion of the serverResource from the apiResourceMetadata
func getServerResourceGroupVersion(apiResourceListGroupVersion, apiResourceGroup, apiResourceVersion string) string {
	var serverResourceGroupVersion string
	if apiResourceGroup == "" && apiResourceVersion == "" {
		serverResourceGroupVersion = apiResourceListGroupVersion
	} else {
		serverResourceGroupVersion = schema.GroupVersion{
			Group:   apiResourceGroup,
			Version: apiResourceVersion,
		}.String()
	}
	return serverResourceGroupVersion
}
