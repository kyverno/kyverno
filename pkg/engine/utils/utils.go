package utils

import (
	"context"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func IsDeleteRequest(ctx engineapi.PolicyContext) bool {
	if ctx == nil {
		return false
	}

	if op := ctx.Operation(); string(op) != "" {
		return op == kyvernov1.Delete
	}

	// if the NewResource is empty, the request is a DELETE
	newResource := ctx.NewResource()
	return IsEmptyUnstructured(&newResource)
}

func IsEmptyUnstructured(u *unstructured.Unstructured) bool {
	if u == nil {
		return true
	}
	if u.Object == nil {
		return true
	}
	return false
}

// ApplyPatches patches given resource with given patches and returns patched document
// return original resource if any error occurs
func ApplyPatches(resource []byte, patches [][]byte) ([]byte, error) {
	if len(patches) == 0 {
		return resource, nil
	}
	joinedPatches := jsonutils.JoinPatches(patches...)
	patch, err := jsonpatch.DecodePatch(joinedPatches)
	if err != nil {
		logging.V(4).Info("failed to decode JSON patch", "patch", patch)
		return resource, err
	}

	patchedDocument, err := patch.Apply(resource)
	if err != nil {
		logging.V(4).Info("failed to apply JSON patch", "patch", patch)
		return resource, err
	}

	logging.V(4).Info("applied JSON patch", "patch", patch)
	return patchedDocument, err
}

// ApplyPatchNew patches given resource with given joined patches
func ApplyPatchNew(resource, patch []byte) ([]byte, error) {
	jsonpatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return resource, err
	}

	patchedResource, err := jsonpatch.Apply(resource)
	if err != nil {
		return resource, err
	}

	return patchedResource, err
}

func TransformConditions(original apiextensions.JSON) (interface{}, error) {
	if original == nil {
		return kyvernov1.AnyAllConditions{}, nil
	}

	switch typedValue := original.(type) {
	case *kyvernov1.AnyAllConditions:
		if typedValue == nil {
			return kyvernov1.AnyAllConditions{}, nil
		}
		return *typedValue.DeepCopy(), nil
	case kyvernov1.AnyAllConditions:
		return *typedValue.DeepCopy(), nil
	case []kyvernov1.Condition: // backwards compatibility
		var copies []kyvernov1.Condition
		for _, condition := range typedValue {
			copies = append(copies, *condition.DeepCopy())
		}
		return copies, nil
	}
	return nil, fmt.Errorf("invalid preconditions")
}

func IsSameRuleResponse(r1 *engineapi.RuleResponse, r2 *engineapi.RuleResponse) bool {
	if r1.Name() != r2.Name() ||
		r1.RuleType() != r2.RuleType() ||
		r1.Message() != r2.Message() ||
		r1.Status() != r2.Status() {
		return false
	}

	return true
}

func IsUpdateRequest(ctx engineapi.PolicyContext) bool {
	// is the OldObject and NewObject are available, the request is an UPDATE
	return (ctx.OldResource().Object != nil && ctx.NewResource().Object != nil) || ctx.Operation() == kyvernov1.Update
}

func IsCreateRequest(ctx engineapi.PolicyContext) bool {
	newResource := ctx.NewResource()
	return ctx.Operation() == kyvernov1.Create ||
		(ctx.OldResource().Object == nil && !IsEmptyUnstructured(&newResource))
}

// RootOwnerCreationTimestamp walks the ownerReference chain of a resource up
// to the root owner and returns its creationTimestamp. Returns the resource's
// own timestamp if it has no owners. Used to detect pre-existing violations
// for managed resources (e.g. pods created by a Deployment that predates the policy).
// Only follows the controller owner (Controller=true). Limits recursion to 10 levels.
func RootOwnerCreationTimestamp(
	ctx context.Context,
	client engineapi.Client,
	resource unstructured.Unstructured,
) (metav1.Time, error) {
	return rootOwnerCreationTimestamp(ctx, client, resource, 0)
}

func rootOwnerCreationTimestamp(
	ctx context.Context,
	client engineapi.Client,
	resource unstructured.Unstructured,
	depth int,
) (metav1.Time, error) {
	const maxDepth = 10
	if depth >= maxDepth {
		return resource.GetCreationTimestamp(), nil
	}
	owners := resource.GetOwnerReferences()
	// find the controller owner (Controller=true); fall back to resource itself if none
	var controllerOwner *metav1.OwnerReference
	for i := range owners {
		if owners[i].Controller != nil && *owners[i].Controller {
			controllerOwner = &owners[i]
			break
		}
	}
	if controllerOwner == nil {
		return resource.GetCreationTimestamp(), nil
	}
	obj, err := client.GetResource(ctx, controllerOwner.APIVersion, controllerOwner.Kind, resource.GetNamespace(), controllerOwner.Name)
	if err != nil {
		return metav1.Time{}, err
	}
	return rootOwnerCreationTimestamp(ctx, client, *obj, depth+1)
}
