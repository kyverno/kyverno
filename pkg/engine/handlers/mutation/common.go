package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type forEachMutator struct {
	logger        logr.Logger
	rule          kyvernov1.Rule
	policyContext engineapi.PolicyContext
	foreach       []kyvernov1.ForEachMutation
	resource      resourceInfo
	nesting       int
	contextLoader engineapi.EngineContextLoader
}

func (f *forEachMutator) mutateForEach(ctx context.Context) ([]patch.Patcher, error) {
	var patchers []patch.Patcher

	for _, foreach := range f.foreach {
		elements, err := engineutils.EvaluateList(foreach.List, f.policyContext.JSONContext())
		if err != nil {
			return nil, err
		}
		p, err := f.mutateElements(ctx, foreach, elements)
		if err != nil {
			return nil, err
		}
		patchers = append(patchers, p...)
	}
	return patchers, nil
}

func (f *forEachMutator) mutateElements(ctx context.Context, foreach kyvernov1.ForEachMutation, elements []interface{}) ([]patch.Patcher, error) {
	f.policyContext.JSONContext().Checkpoint()
	defer f.policyContext.JSONContext().Restore()

	patchedResource := f.resource
	var patchers []patch.Patcher
	reverse := false
	if foreach.RawPatchStrategicMerge != nil {
		reverse = true
	} else if foreach.Order != nil && *foreach.Order == kyvernov1.Descending {
		reverse = true
	}
	if reverse {
		engineutils.InvertedElement(elements)
	}

	for index, element := range elements {
		if element == nil {
			continue
		}
		if reverse {
			index = len(elements) - 1 - index
		}
		f.policyContext.JSONContext().Reset()
		policyContext := f.policyContext.Copy()

		falseVar := false
		if err := engineutils.AddElementToContext(policyContext, element, index, f.nesting, &falseVar); err != nil {
			return nil, err
		}

		if err := f.contextLoader(ctx, foreach.Context, policyContext.JSONContext()); err != nil {
			return nil, err
		}

		preconditionsPassed, err := internal.CheckPreconditions(f.logger, policyContext.JSONContext(), foreach.AnyAllConditions)
		if err != nil {
			return nil, err
		}

		if !preconditionsPassed {
			f.logger.Info("mutate.foreach.preconditions not met", "elementIndex", index)
			continue
		}

		if foreach.ForEachMutation != nil {
			nestedForEach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](foreach.ForEachMutation)
			if err != nil {
				return nil, err
			}

			m := &forEachMutator{
				rule:          f.rule,
				policyContext: f.policyContext,
				resource:      patchedResource,
				logger:        f.logger,
				foreach:       nestedForEach,
				nesting:       f.nesting + 1,
				contextLoader: f.contextLoader,
			}

			p, err := m.mutateForEach(ctx)
			if err != nil {
				return nil, err
			}
			patchers = append(patchers, p...)
		} else {
			p, err := mutate.ForEach(f.logger, foreach, policyContext)
			if err != nil {
				return nil, err
			}
			patchers = append(patchers, p)
		}
	}
	return patchers, nil
}

// func buildRuleResponse(rule *kyvernov1.Rule, mutateResp *mutate.Response, info resourceInfo) *engineapi.RuleResponse {
// 	message := mutateResp.Message
// 	if mutateResp.Status == engineapi.RuleStatusPass {
// 		message = buildSuccessMessage(info.unstructured)
// 	}
// 	resp := engineapi.NewRuleResponse(
// 		rule.Name,
// 		engineapi.Mutation,
// 		message,
// 		mutateResp.Status,
// 	)
// 	if mutateResp.Status == engineapi.RuleStatusPass {
// 		resp = resp.WithPatches(patch.ConvertPatches(mutateResp.Patches...)...)
// 		// TODO
// 		// if len(rule.Mutation.Targets) != 0 {
// 		// 	resp = resp.WithPatchedTarget(&mutateResp.PatchedResource, info.parentResourceGVR, info.subresource)
// 		// }
// 	}
// 	return resp
// }

// func buildSuccessMessage(r unstructured.Unstructured) string {
// 	if r.Object == nil {
// 		return "mutated resource"
// 	}
// 	if r.GetNamespace() == "" {
// 		return fmt.Sprintf("mutated %s/%s", r.GetKind(), r.GetName())
// 	}
// 	return fmt.Sprintf("mutated %s/%s in namespace %s", r.GetKind(), r.GetName(), r.GetNamespace())
// }

func applyPatchers(logger logr.Logger, resource unstructured.Unstructured, rule kyvernov1.Rule, patchers ...patch.Patcher) (unstructured.Unstructured, *engineapi.RuleResponse) {
	if len(patchers) == 0 {
		return resource, engineapi.RuleSkip(rule.Name, engineapi.Mutation, "no patches")
	}
	// apply patchers
	resourceBytes, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to marshal resource", err)
	}
	var allPatches []jsonpatch.JsonPatchOperation
	for _, patcher := range patchers {
		patchedBytes, patches, err := patcher.Patch(logger, resourceBytes)
		if err != nil {
			logger.Error(err, "failed to patch resource")
			return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to patch resource", err)
		}
		resourceBytes = patchedBytes
		allPatches = append(allPatches, patches...)
	}
	if len(allPatches) == 0 {
		return resource, engineapi.RuleSkip(rule.Name, engineapi.Mutation, "no patches")
	}
	err = resource.UnmarshalJSON(resourceBytes)
	if err != nil {
		logger.Error(err, "failed to unmarshal resource")
		return resource, engineapi.RuleError(rule.Name, engineapi.Mutation, "failed to unmarshal resource", err)
	}
	return resource, engineapi.RulePass(rule.Name, engineapi.Mutation, "TODO").WithPatches(patch.ConvertPatches(allPatches...)...)
}
