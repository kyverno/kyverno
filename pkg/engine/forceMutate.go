package engine

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ForceMutate does not check any conditions, it simply mutates the given resource
// It is used to validate mutation logic, and for tests.
func ForceMutate(
	ctx context.Interface,
	logger logr.Logger,
	policy kyvernov1.PolicyInterface,
	resource unstructured.Unstructured,
) (unstructured.Unstructured, error) {
	logger = internal.LoggerWithPolicy(logger, policy)
	logger = internal.LoggerWithResource(logger, "resource", resource)
	// logger := logging.WithName("EngineForceMutate").WithValues("policy", policy.GetName(), "kind", resource.GetKind(),
	// 	"namespace", resource.GetNamespace(), "name", resource.GetName())

	patchedResource := resource
	// TODO: if we apply autogen, tests will fail
	spec := policy.GetSpec()
	for _, rule := range spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		logger := internal.LoggerWithRule(logger, rule)

		ruleCopy := rule.DeepCopy()
		removeConditions(ruleCopy)
		r, err := variables.SubstituteAllForceMutate(logger, ctx, *ruleCopy)
		if err != nil {
			return resource, err
		}

		if r.Mutation.ForEachMutation != nil {
			patchedResource, err = applyForEachMutate(r.Name, r.Mutation.ForEachMutation, patchedResource, logger)
			if err != nil {
				return patchedResource, err
			}
		} else {
			m := r.Mutation
			patchedResource, err = applyPatches(m.GetPatchStrategicMerge(), m.PatchesJSON6902, patchedResource, logger)
			if err != nil {
				return patchedResource, err
			}
		}
	}

	return patchedResource, nil
}

func applyForEachMutate(name string, foreach []kyvernov1.ForEachMutation, resource unstructured.Unstructured, logger logr.Logger) (patchedResource unstructured.Unstructured, err error) {
	patchedResource = resource
	for _, fe := range foreach {
		fem := fe.GetForEachMutation()
		if len(fem) > 0 {
			return applyForEachMutate(name, fem, patchedResource, logger)
		}

		patchedResource, err = applyPatches(fe.GetPatchStrategicMerge(), fe.PatchesJSON6902, patchedResource, logger)
		if err != nil {
			return resource, err
		}
	}

	return patchedResource, nil
}

func applyPatches(mergePatch apiextensions.JSON, jsonPatch string, resource unstructured.Unstructured, logger logr.Logger) (unstructured.Unstructured, error) {
	patcher := mutate.NewPatcher(mergePatch, jsonPatch)
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		return resource, err
	}
	resourceBytes, err = patcher.Patch(logger, resourceBytes)
	if err != nil {
		return resource, err
	}
	if err := resource.UnmarshalJSON(resourceBytes); err != nil {
		return resource, err
	}
	return resource, err
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
