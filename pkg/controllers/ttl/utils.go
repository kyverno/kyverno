package ttl

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	checker "github.com/kyverno/kyverno/pkg/auth/checker"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
)

func discoverResources(logger logr.Logger, discoveryClient discovery.DiscoveryInterface) ([]schema.GroupVersionResource, error) {
	var resources []schema.GroupVersionResource
	apiResourceList, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return nil, err
		}
		// the error should be recoverable, let's log missing groups and process the partial results we received
		err := err.(*discovery.ErrGroupDiscoveryFailed)
		for gv, err := range err.Groups {
			// Handling the specific group error
			logger.Error(err, "error in discovering group", "gv", gv)
		}
	}
	for _, apiResourceList := range apiResourceList {
		for _, apiResource := range apiResourceList.APIResources {
			if sets.NewString(apiResource.Verbs...).HasAll("list", "watch", "delete") {
				groupVersion, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
				if err != nil {
					return resources, err
				}
				resources = append(resources, groupVersion.WithResource(apiResource.Name))
			}
		}
	}
	return resources, nil
}

func HasResourcePermissions(logger logr.Logger, resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(context.TODO(), s, resource.Group, resource.Version, resource.Resource, "", "", "watch", "list", "delete")
	if err != nil {
		logger.Error(err, "failed to check permissions")
		return false
	}
	return can
}

func parseDeletionTime(metaObj metav1.Object, deletionTime *time.Time, ttlValue string) error {
	ttlDuration, err := strfmt.ParseDuration(ttlValue)
	if err == nil {
		creationTime := metaObj.GetCreationTimestamp().Time
		*deletionTime = creationTime.Add(ttlDuration)
	} else {
		// Try parsing ttlValue as a time in ISO 8601 format
		*deletionTime, err = time.Parse(kyverno.ValueTtlDateTimeLayout, ttlValue)
		if err != nil {
			*deletionTime, err = time.Parse(kyverno.ValueTtlDateLayout, ttlValue)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
