package generate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type generator struct {
	client           dclient.Interface
	logger           logr.Logger
	policyContext    engineapi.PolicyContext
	policy           kyvernov1.PolicyInterface
	rule             kyvernov1.Rule
	contextEntries   []kyvernov1.ContextEntry
	anyAllConditions any
	trigger          unstructured.Unstructured
	forEach          []kyvernov1.ForEachGeneration
	pattern          kyvernov1.GeneratePattern
	contextLoader    engineapi.EngineContextLoader
}

func newGenerator(client dclient.Interface,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
	contextEntries []kyvernov1.ContextEntry,
	anyAllConditions any,
	trigger unstructured.Unstructured,
	pattern kyvernov1.GeneratePattern,
	contextLoader engineapi.EngineContextLoader,
) *generator {
	return &generator{
		client:           client,
		logger:           logger,
		policyContext:    policyContext,
		policy:           policy,
		rule:             rule,
		contextEntries:   contextEntries,
		anyAllConditions: anyAllConditions,
		trigger:          trigger,
		pattern:          pattern,
		contextLoader:    contextLoader,
	}
}

func newForeachGenerator(client dclient.Interface,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	policy kyvernov1.PolicyInterface,
	rule kyvernov1.Rule,
	contextEntries []kyvernov1.ContextEntry,
	anyAllConditions any,
	trigger unstructured.Unstructured,
	forEach []kyvernov1.ForEachGeneration,
	contextLoader engineapi.EngineContextLoader,
) *generator {
	return &generator{
		client:           client,
		logger:           logger,
		policyContext:    policyContext,
		policy:           policy,
		rule:             rule,
		contextEntries:   contextEntries,
		anyAllConditions: anyAllConditions,
		trigger:          trigger,
		forEach:          forEach,
		contextLoader:    contextLoader,
	}
}

func (g *generator) generate() ([]kyvernov1.ResourceSpec, error) {
	responses := []generateResponse{}
	var err error
	var newGenResources []kyvernov1.ResourceSpec

	if err := g.loadContext(context.TODO()); err != nil {
		return newGenResources, fmt.Errorf("failed to load context: %v", err)
	}

	typeConditions, err := engineutils.TransformConditions(g.anyAllConditions)
	if err != nil {
		return newGenResources, fmt.Errorf("failed to parse preconditions: %v", err)
	}

	preconditionsPassed, msg, err := variables.EvaluateConditions(g.logger, g.policyContext.JSONContext(), typeConditions)
	if err != nil {
		return newGenResources, fmt.Errorf("failed to evaluate preconditions: %v", err)
	}

	if !preconditionsPassed {
		g.logger.V(2).Info("preconditions not met", "msg", msg)
		return newGenResources, nil
	}

	pattern, err := variables.SubstituteAllInType(g.logger, g.policyContext.JSONContext(), &g.pattern)
	if err != nil {
		g.logger.Error(err, "variable substitution failed for rule", "rule", g.rule.Name)
		return nil, err
	}

	target := pattern.ResourceSpec
	logger := g.logger.WithValues("target", target.String())

	if pattern.Clone.Name != "" {
		resp := manageClone(logger.WithValues("type", "clone"), target, kyvernov1.ResourceSpec{}, g.policy.GetSpec().UseServerSideApply, *pattern, g.client)
		responses = append(responses, resp)
	} else if len(pattern.CloneList.Kinds) != 0 {
		responses = manageCloneList(logger.WithValues("type", "cloneList"), target.GetNamespace(), g.policy.GetSpec().UseServerSideApply, *pattern, g.client)
	} else {
		resp := manageData(logger.WithValues("type", "data"), target, pattern.RawData, g.rule.Generation.Synchronize, g.client)
		responses = append(responses, resp)
	}

	for _, response := range responses {
		targetMeta := response.GetTarget()
		if response.GetError() != nil {
			logger.Error(response.GetError(), "failed to generate resource", "mode", response.GetAction())
			return newGenResources, err
		}

		if response.GetAction() == Skip {
			continue
		}

		logger.V(3).Info("applying generate rule", "mode", response.GetAction())
		if response.GetData() == nil && response.GetAction() == Update {
			logger.V(4).Info("no changes required for generate target resource")
			return newGenResources, nil
		}

		newResource := &unstructured.Unstructured{}
		newResource.SetUnstructuredContent(response.GetData())
		newResource.SetName(targetMeta.GetName())
		newResource.SetNamespace(targetMeta.GetNamespace())
		if newResource.GetKind() == "" {
			newResource.SetKind(targetMeta.GetKind())
		}

		newResource.SetAPIVersion(targetMeta.GetAPIVersion())
		common.ManageLabels(newResource, g.trigger, g.policy, g.rule.Name)
		if response.GetAction() == Create {
			newResource.SetResourceVersion("")
			if g.policy.GetSpec().UseServerSideApply {
				_, err = g.client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
			} else {
				_, err = g.client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
			}
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return newGenResources, err
				}
			}
			logger.V(2).Info("created generate target resource")
			newGenResources = append(newGenResources, targetMeta)
		} else if response.GetAction() == Update {
			generatedObj, err := g.client.GetResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName())
			if err != nil {
				logger.V(2).Info("creating new target due to the failure when fetching", "err", err.Error())
				if g.policy.GetSpec().UseServerSideApply {
					_, err = g.client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
				} else {
					_, err = g.client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
				}
				if err != nil {
					return newGenResources, err
				}
				newGenResources = append(newGenResources, targetMeta)
			} else {
				if !g.rule.Generation.Synchronize {
					logger.V(4).Info("synchronize disabled, skip syncing changes")
					continue
				}
				if err := validate.MatchPattern(logger, newResource.Object, generatedObj.Object); err == nil {
					if err := validate.MatchPattern(logger, generatedObj.Object, newResource.Object); err == nil {
						logger.V(4).Info("patterns match, skipping updates")
						continue
					}
				}

				logger.V(4).Info("updating existing resource")
				if targetMeta.GetAPIVersion() == "" {
					generatedResourceAPIVersion := generatedObj.GetAPIVersion()
					newResource.SetAPIVersion(generatedResourceAPIVersion)
				}
				if targetMeta.GetNamespace() == "" {
					newResource.SetNamespace("default")
				}

				if g.policy.GetSpec().UseServerSideApply {
					_, err = g.client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
				} else {
					_, err = g.client.UpdateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
				}
				if err != nil {
					logger.Error(err, "failed to update resource")
					return newGenResources, err
				}
			}
			logger.V(3).Info("updated generate target resource")
		}
	}
	return newGenResources, nil
}

func (g *generator) generateForeach() ([]kyvernov1.ResourceSpec, error) {
	var errors []error
	var genResources []kyvernov1.ResourceSpec

	for i, foreach := range g.forEach {
		elements, err := engineutils.EvaluateList(foreach.List, g.policyContext.JSONContext())
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to evaluate %v foreach list: %v", i, err))
			continue
		}
		gen, err := g.generateElements(foreach, elements, nil)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to process %v foreach in rule %s: %v", i, g.rule.Name, err))
		}
		if gen != nil {
			genResources = append(genResources, gen...)
		}
	}
	return genResources, multierr.Combine(errors...)
}

func (g *generator) generateElements(foreach kyvernov1.ForEachGeneration, elements []interface{}, elementScope *bool) ([]kyvernov1.ResourceSpec, error) {
	var errors []error
	var genResources []kyvernov1.ResourceSpec
	g.policyContext.JSONContext().Checkpoint()
	defer g.policyContext.JSONContext().Restore()

	for index, element := range elements {
		if element == nil {
			continue
		}

		g.policyContext.JSONContext().Reset()
		policyContext := g.policyContext.Copy()
		if err := engineutils.AddElementToContext(policyContext, element, index, 0, elementScope); err != nil {
			g.logger.Error(err, "")
			errors = append(errors, fmt.Errorf("failed to add %v element to context: %v", index, err))
			continue
		}

		gen, err := newGenerator(g.client,
			g.logger,
			policyContext,
			g.policy,
			g.rule,
			foreach.Context,
			foreach.AnyAllConditions,
			g.trigger,
			foreach.GeneratePattern,
			g.contextLoader).
			generate()
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to process %v element: %v", index, err))
		}
		if gen != nil {
			genResources = append(genResources, gen...)
		}
	}
	return genResources, multierr.Combine(errors...)
}

func (g *generator) loadContext(ctx context.Context) error {
	if err := g.contextLoader(ctx, g.contextEntries, g.policyContext.JSONContext()); err != nil {
		if _, ok := err.(gojmespath.NotFoundError); ok {
			g.logger.V(3).Info("failed to load context", "reason", err.Error())
		} else {
			g.logger.Error(err, "failed to load context")
		}
		return err
	}
	return nil
}
