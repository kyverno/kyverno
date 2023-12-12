package utils

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func IsDeleteRequest(ctx engineapi.PolicyContext) bool {
	newResource := ctx.NewResource()
	// if the OldResource is not empty, and the NewResource is empty, the request is a DELETE
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
	// conditions are currently in the form of []interface{}
	oldConditions, err := apiutils.ApiextensionsJsonToKyvernoConditions(original)
	if err != nil {
		return nil, err
	}
	switch typedValue := oldConditions.(type) {
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
	return ctx.OldResource().Object != nil && ctx.NewResource().Object != nil
}
