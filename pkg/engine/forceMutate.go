package engine

import (
	"encoding/json"
	"fmt"
	"regexp"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/mutate"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func mutateResourceWithOverlay(resource unstructured.Unstructured, overlay interface{}) (unstructured.Unstructured, error) {
	patches, err := mutate.MutateResourceWithOverlay(resource.UnstructuredContent(), overlay)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	if len(patches) == 0 {
		return resource, nil
	}

	// convert to RAW
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		return unstructured.Unstructured{}, err
	}

	var patchResource []byte
	patchResource, err = utils.ApplyPatches(resourceRaw, patches)
	if err != nil {
		return unstructured.Unstructured{}, err
	}

	resource = unstructured.Unstructured{}
	err = resource.UnmarshalJSON(patchResource)
	if err != nil {
		return unstructured.Unstructured{}, err
	}

	return resource, nil
}

// ForceMutate does not check any conditions, it simply mutates the given resource
func ForceMutate(ctx context.EvalInterface, policy kyverno.ClusterPolicy, resource unstructured.Unstructured) (unstructured.Unstructured, error) {
	var err error
	for _, rule := range policy.Spec.Rules {
		if !rule.HasMutate() {
			continue
		}

		mutation := rule.Mutation.DeepCopy()

		if mutation.Overlay != nil {
			overlay := mutation.Overlay
			if ctx != nil {
				if overlay, err = variables.SubstituteVars(log.Log, ctx, overlay); err != nil {
					return unstructured.Unstructured{}, err
				}
			} else {
				overlay = replaceSubstituteVariables(overlay)
			}

			resource, err = mutateResourceWithOverlay(resource, overlay)
			if err != nil {
				return unstructured.Unstructured{}, fmt.Errorf("could not mutate resource with overlay on rule %v:%v", rule.Name, err)
			}
		}

		if rule.Mutation.Patches != nil {
			var resp response.RuleResponse
			resp, resource = mutate.ProcessPatches(log.Log, rule, resource)
			if !resp.Success {
				return unstructured.Unstructured{}, fmt.Errorf(resp.Message)
			}
		}
	}

	return resource, nil
}

func replaceSubstituteVariables(overlay interface{}) interface{} {
	overlayRaw, err := json.Marshal(overlay)
	if err != nil {
		return overlay
	}

	regex := regexp.MustCompile(`\{\{([^{}]*)\}\}`)
	for {
		if len(regex.FindAllStringSubmatch(string(overlayRaw), -1)) > 0 {
			overlayRaw = regex.ReplaceAll(overlayRaw, []byte(`placeholderValue`))
		} else {
			break
		}
	}

	var output interface{}
	err = json.Unmarshal(overlayRaw, &output)
	if err != nil {
		return overlay
	}

	return output
}
