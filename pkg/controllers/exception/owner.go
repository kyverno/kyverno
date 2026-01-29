package exception

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// topLevelControllers are controller kinds that are considered "root owners"
// and should not be traversed further up the owner chain.
var topLevelControllers = sets.New(
	"Deployment",
	"StatefulSet",
	"DaemonSet",
	"CronJob",
	"Job",
	"ReplicaSet",
	"ReplicationController",
)

// resolveRootOwner traverses the ownerReferences chain of a resource to find the
// top-level controller that manages it. For example:
//
//	Pod → ReplicaSet → Deployment (returns Deployment)
//	Pod (no owner) → returns Pod itself
//	Pod → StatefulSet → returns StatefulSet
//
// It stops traversal when it encounters a top-level controller kind or when
// no more owner references exist.
func resolveRootOwner(ctx context.Context, client dclient.Interface, resource unstructured.Unstructured) (ownerInfo, error) {
	current := ownerInfo{
		kind:      resource.GetKind(),
		name:      resource.GetName(),
		namespace: resource.GetNamespace(),
	}

	// If the resource itself is a top-level controller, return it directly.
	if topLevelControllers.Has(current.kind) {
		return current, nil
	}

	visited := sets.New[string]()
	obj := &resource

	for {
		key := fmt.Sprintf("%s/%s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
		if visited.Has(key) {
			// Circular reference protection
			break
		}
		visited.Insert(key)

		owners := obj.GetOwnerReferences()
		if len(owners) == 0 {
			// No owner — current resource is the root
			return ownerInfo{
				kind:      obj.GetKind(),
				name:      obj.GetName(),
				namespace: obj.GetNamespace(),
			}, nil
		}

		// Use the first controller owner reference
		var found bool
		for _, ref := range owners {
			if ref.Controller != nil && *ref.Controller {
				current = ownerInfo{
					kind:      ref.Kind,
					name:      ref.Name,
					namespace: obj.GetNamespace(),
				}
				found = true

				// If the owner is a top-level controller, stop here
				if topLevelControllers.Has(ref.Kind) {
					return current, nil
				}

				// Fetch the owner to continue traversal
				owner, err := client.GetResource(ctx, ref.APIVersion, ref.Kind, obj.GetNamespace(), ref.Name)
				if err != nil {
					// If we can't fetch the owner, return what we have so far
					return current, nil
				}
				obj = owner
				break
			}
		}
		if !found {
			// No controller owner reference found
			return ownerInfo{
				kind:      obj.GetKind(),
				name:      obj.GetName(),
				namespace: obj.GetNamespace(),
			}, nil
		}
	}

	return current, nil
}
