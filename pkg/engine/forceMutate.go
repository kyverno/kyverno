package engine

import (
	"fmt"

	"github.com/ghodss/yaml"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func mutateResourceWithOverlay(resource unstructured.Unstructured, overlay interface{}) (unstructured.Unstructured, error) {
	logger := log.Log.WithValues("resource", resource.GetKind(), "overlay", overlay)
	patches, err := mutate.MutateResourceWithOverlay(resource.UnstructuredContent(), overlay)
	if err != nil {
		logger.V(4).Info("failed to mutate resource with overlay")
		return resource, err
	}

	if len(patches) == 0 {
		return resource, nil
	}

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		logger.V(4).Info("failed to marshall resource JSON")
		return resource, err
	}

	var patchResource []byte
	patchResource, err = utils.ApplyPatches(resourceRaw, patches)
	if err != nil {
		logger.V(4).Info("failed to apply patches")
		return resource, err
	}

	resource = unstructured.Unstructured{}
	err = resource.UnmarshalJSON(patchResource)
	if err != nil {
		logger.V(4).Info("failed to unmarshal patched resource JSON")
		return resource, err
	}

	logger.V(4).Info("mutated resource with overlay")
	return resource, nil
}

// ForceMutate does not check any conditions, it simply mutates the given resource
// It is used to validate mutation logic, and for tests.
func ForceMutate(ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (unstructured.Unstructured, error) {
	var err error
	logger := log.Log.WithName("EngineForceMutate").WithValues("policy", policy.Name, "kind", resource.GetKind(),
		"namespace", resource.GetNamespace(), "name", resource.GetName())

	for _, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		rule, err = variables.SubstituteAllForceMutate(log.Log, ctx, rule)
		if err != nil {
			return unstructured.Unstructured{}, err
		}

		mutation := rule.Mutation.DeepCopy()

		if mutation.Overlay != nil {
			overlay := mutation.Overlay

			resource, err = mutateResourceWithOverlay(resource, overlay)
			if err != nil {
				detailedErr := fmt.Errorf("failed to mutate resource %s with overlay rule %v:%v", resource.GetKind(), rule.Name, err)
				return unstructured.Unstructured{}, detailedErr
			}
		}

		if rule.Mutation.Patches != nil {
			var resp response.RuleResponse
			resp, resource = mutate.ProcessPatches(logger.WithValues("rule", rule.Name), rule.Name, rule.Mutation, resource)
			if resp.Status != response.RuleStatusPass {
				return unstructured.Unstructured{}, fmt.Errorf(resp.Message)
			}
		}

		if rule.Mutation.PatchStrategicMerge != nil {
			var resp response.RuleResponse
			resp, resource = mutate.ProcessStrategicMergePatch(rule.Name, rule.Mutation.PatchStrategicMerge, resource, logger.WithValues("rule", rule.Name))
			if resp.Status != response.RuleStatusPass {
				return unstructured.Unstructured{}, fmt.Errorf(resp.Message)
			}
		}

		if rule.Mutation.PatchesJSON6902 != "" {
			var resp response.RuleResponse
			jsonPatches, err := yaml.YAMLToJSON([]byte(rule.Mutation.PatchesJSON6902))
			if err != nil {
				return unstructured.Unstructured{}, err
			}

			resp, resource = mutate.ProcessPatchJSON6902(rule.Name, jsonPatches, resource, logger.WithValues("rule", rule.Name))
			if resp.Status != response.RuleStatusPass {
				return unstructured.Unstructured{}, fmt.Errorf(resp.Message)
			}
		}
	}

	return resource, nil
}
