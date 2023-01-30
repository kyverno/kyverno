package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ForceMutate does not check any conditions, it simply mutates the given resource
// It is used to validate mutation logic, and for tests.
func ForceMutate(ctx context.Interface, policy kyvernov1.PolicyInterface, resource unstructured.Unstructured) (unstructured.Unstructured, error) {
	logger := logging.WithName("EngineForceMutate").WithValues("policy", policy.GetName(), "kind", resource.GetKind(),
		"namespace", resource.GetNamespace(), "name", resource.GetName())

	patchedResource := resource
	// TODO: if we apply autogen, tests will fail
	spec := policy.GetSpec()
	for _, rule := range spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		ruleCopy := rule.DeepCopy()
		removeConditions(ruleCopy)
		r, err := variables.SubstituteAllForceMutate(logger, ctx, *ruleCopy)
		if err != nil {
			return resource, err
		}

		if r.Mutation.ForEachMutation != nil {
			patchedResource, err = applyForEachMutate(r.Name, r.Mutation.ForEachMutation, patchedResource, ctx, logger)
			if err != nil {
				return patchedResource, err
			}
		} else {
			m := r.Mutation
			patchedResource, err = applyPatches(r.Name, m.GetPatchStrategicMerge(), m.PatchesJSON6902, patchedResource, ctx, logger)
			if err != nil {
				return patchedResource, err
			}
		}
	}

	return patchedResource, nil
}

func applyForEachMutate(name string, foreach []kyvernov1.ForEachMutation, resource unstructured.Unstructured, ctx context.Interface, logger logr.Logger) (patchedResource unstructured.Unstructured, err error) {
	patchedResource = resource
	for _, fe := range foreach {
		if fe.ForEachMutation != nil {
			nestedForEach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](fe.ForEachMutation)
			if err != nil {
				return patchedResource, errors.Wrapf(err, "failed to deserialize foreach")
			}

			return applyForEachMutate(name, nestedForEach, patchedResource, ctx, logger)
		}

		patchedResource, err = applyPatches(name, fe.GetPatchStrategicMerge(), fe.PatchesJSON6902, patchedResource, ctx, logger)
		if err != nil {
			return resource, err
		}
	}

	return patchedResource, nil
}

func applyPatches(name string, mergePatch apiextensions.JSON, jsonPatch string, resource unstructured.Unstructured, ctx context.Interface, logger logr.Logger) (unstructured.Unstructured, error) {
	patcher := mutate.NewPatcher(name, mergePatch, jsonPatch, resource, ctx, logger)
	resp, mutatedResource := patcher.Patch()
	if resp.Status != engineapi.RuleStatusPass {
		return mutatedResource, fmt.Errorf("mutate status %q: %s", resp.Status, resp.Message)
	}

	return mutatedResource, nil
}

// removeConditions mutates the rule to remove AnyAllConditions
func removeConditions(rule *kyvernov1.Rule) {
	if rule.GetAnyAllConditions() != nil {
		rule.SetAnyAllConditions(nil)
	}

	for i, fem := range rule.Mutation.ForEachMutation {
		if fem.AnyAllConditions != nil {
			rule.Mutation.ForEachMutation[i].AnyAllConditions = nil
		}
	}
}
