package engine

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ForceMutate does not check any conditions, it simply mutates the given resource
// It is used to validate mutation logic, and for tests.
func ForceMutate(ctx *context.Context, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (unstructured.Unstructured, error) {
	logger := log.Log.WithName("EngineForceMutate").WithValues("policy", policy.Name, "kind", resource.GetKind(),
		"namespace", resource.GetNamespace(), "name", resource.GetName())

	patchedResource := resource
	for _, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		r, err := variables.SubstituteAllForceMutate(logger, ctx, removeConditions(rule))
		if err != nil {
			return resource, err
		}

		if r.Mutation.ForEachMutation != nil {
			for _, foreach := range r.Mutation.ForEachMutation {
				patcher := mutate.NewPatcher(r.Name, foreach.PatchStrategicMerge, foreach.PatchesJSON6902, patchedResource, ctx, logger)
				resp, mutatedResource := patcher.Patch()
				if resp.Status != response.RuleStatusPass {
					return patchedResource, fmt.Errorf("foreach mutate result %q at index %d: %s", resp.Status.String(), i, resp.Message)
				}

				patchedResource = mutatedResource
			}
		} else {
			m := r.Mutation
			patcher := mutate.NewPatcher(r.Name, m.PatchStrategicMerge, m.PatchesJSON6902, patchedResource, ctx, logger)
			resp, mutatedResource := patcher.Patch()
			if resp.Status != response.RuleStatusPass {
				return patchedResource, fmt.Errorf("mutate result %q: %s", resp.Status.String(), resp.Message)
			}

			patchedResource = mutatedResource
		}
	}

	return patchedResource, nil
}

func removeConditions(rule kyverno.Rule) kyverno.Rule {
	ruleCopy := rule.DeepCopy()

	if ruleCopy.AnyAllConditions != nil {
		ruleCopy.AnyAllConditions = nil
	}
	for i, fem := range ruleCopy.Mutation.ForEachMutation {
		if fem.AnyAllConditions != nil {
			ruleCopy.Mutation.ForEachMutation[i].AnyAllConditions = nil
		}
	}

	return *ruleCopy
}
