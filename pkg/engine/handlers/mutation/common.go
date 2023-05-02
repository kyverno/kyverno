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
