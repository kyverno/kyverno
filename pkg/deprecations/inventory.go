package deprecations

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/metadata"
)

// LegacyPolicyResources maps each legacy kyverno.io kind to the resource the inventory lists.
var LegacyPolicyResources = map[string]schema.GroupVersionResource{
	"ClusterPolicy":        {Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"},
	"Policy":               {Group: "kyverno.io", Version: "v1", Resource: "policies"},
	"CleanupPolicy":        {Group: "kyverno.io", Version: "v2", Resource: "cleanuppolicies"},
	"ClusterCleanupPolicy": {Group: "kyverno.io", Version: "v2", Resource: "clustercleanuppolicies"},
	"PolicyException":      {Group: "kyverno.io", Version: "v2", Resource: "policyexceptions"},
}

// MetadataKindCounters returns one counter per kind, backed by a metadata-only list across all
// namespaces. A NotFound (CRD not installed) counts as zero; any other error is returned as is.
func MetadataKindCounters(client metadata.Interface, kinds ...string) map[string]KindCounter {
	counters := make(map[string]KindCounter, len(kinds))
	for _, kind := range kinds {
		gvr, ok := LegacyPolicyResources[kind]
		if !ok {
			panic(fmt.Sprintf("unknown legacy policy kind %q", kind)) // programming error, both callers use literals
		}
		counters[kind] = func(ctx context.Context) (int, error) {
			list, err := client.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return 0, nil
				}
				return 0, err
			}
			return len(list.Items), nil
		}
	}
	return counters
}
