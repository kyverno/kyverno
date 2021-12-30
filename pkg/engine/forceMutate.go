package engine

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ForceMutate does not check any conditions, it simply mutates the given resource
// It is used to validate mutation logic, and for tests.
func ForceMutate(ctx *context.Context, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (unstructured.Unstructured, error) {
	logger := log.Log.WithName("EngineForceMutate").WithValues("policy", policy.Name, "kind", resource.GetKind(),
		"namespace", resource.GetNamespace(), "name", resource.GetName())

	if ctx == nil {
		ctx = context.NewContext()
		ctx.AddResourceAsObject(resource.Object)
	}

	patchedResource := resource
	for i, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		ruleCopy := removeConditions(&policy.Spec.Rules[i])
		if ruleCopy.Mutation.ForEachMutation != nil {
			for _, foreach := range ruleCopy.Mutation.ForEachMutation {
				mutateResp := mutate.ForEach(rule.Name, foreach, ctx, patchedResource, logger)
				if mutateResp.Status == response.RuleStatusError {
					return patchedResource, fmt.Errorf("%s", mutateResp.Message)
				}

				patchedResource = mutateResp.PatchedResource
			}
		} else {
			mutateResp := mutate.Mutate(ruleCopy, ctx, patchedResource, logger)
			if mutateResp.Status == response.RuleStatusError {
				return patchedResource, fmt.Errorf("%s", mutateResp.Message)
			}

			patchedResource = mutateResp.PatchedResource
		}
	}

	return patchedResource, nil
}

func removeConditions(rule *kyverno.Rule) *kyverno.Rule {
	ruleCopy := rule.DeepCopy()

	if ruleCopy.AnyAllConditions != nil {
		ruleCopy.AnyAllConditions = nil
	}

	for i, fem := range ruleCopy.Mutation.ForEachMutation {
		if fem.AnyAllConditions != nil {
			ruleCopy.Mutation.ForEachMutation[i].AnyAllConditions = nil
		}
	}

	return ruleCopy
}
