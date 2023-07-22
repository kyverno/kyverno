package ttlcontroller

import (
	"context"
	"log"
	"time"

	"github.com/go-logr/logr"
	checker "github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

func discoverResources(discoveryClient discovery.DiscoveryInterface) ([]schema.GroupVersionResource, error) {
	var resources []schema.GroupVersionResource

	apiResourceList, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) { // the error should be recoverable, let's log missing groups and process the partial results we received
			err := err.(*discovery.ErrGroupDiscoveryFailed)
			for gv, groupErr := range err.Groups {
				// Handling the specific group error
				log.Printf("Error in discovering group %s: %v", gv.String(), groupErr)
			}
		} else { // if not a discovery error we should return early
			// Handling other non-group-specific errors
			return nil, err
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

func hasResourcePermissions(resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(context.TODO(), s, resource.Group, resource.Version, resource.Resource, "", "", "watch", "list", "delete")
	if err != nil {
		log.Println("failed to check permissions", err)
		return false
	}
	return can
}

func parseDeletionTime(metaObj metav1.Object, deletionTime *time.Time, ttlValue string) error {
	ttlDuration, err := time.ParseDuration(ttlValue)
	if err == nil {
		creationTime := metaObj.GetCreationTimestamp().Time
		*deletionTime = creationTime.Add(ttlDuration)
	} else {
		layoutRFCC := "2006-01-02T150405Z"
		// Try parsing ttlValue as a time in ISO 8601 format
		*deletionTime, err = time.Parse(layoutRFCC, ttlValue)
		if err != nil {
			layoutCustom := "2006-01-02"
			*deletionTime, err = time.Parse(layoutCustom, ttlValue)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func CreateLogger(name string) logr.Logger {
	return logging.WithName(name)
}
