package engine

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/logging"
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
			for i, foreach := range r.Mutation.ForEachMutation {
				patcher := mutate.NewPatcher(r.Name, foreach.GetPatchStrategicMerge(), foreach.PatchesJSON6902, patchedResource, ctx, logger)
				resp, mutatedResource := patcher.Patch()
				if resp.Status != response.RuleStatusPass {
					return patchedResource, fmt.Errorf("foreach mutate result %q at index %d: %s", resp.Status.String(), i, resp.Message)
				}

				patchedResource = mutatedResource
			}
		} else {
			m := r.Mutation
			patcher := mutate.NewPatcher(r.Name, m.GetPatchStrategicMerge(), m.PatchesJSON6902, patchedResource, ctx, logger)
			resp, mutatedResource := patcher.Patch()
			if resp.Status != response.RuleStatusPass {
				return patchedResource, fmt.Errorf("mutate result %q: %s", resp.Status.String(), resp.Message)
			}

			patchedResource = mutatedResource
		}
	}

	return patchedResource, nil
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
