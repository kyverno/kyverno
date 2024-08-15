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
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
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
	forEach          []kyvernov1.ForEachValidation
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
	contextLoader engineapi.EngineContextLoader) *generator {
	return &generator{
		client:           client,
		logger:           logger,
		policyContext:    policyContext,
		policy:           policy,
		rule:             rule,
		contextEntries:   contextEntries,
		anyAllConditions: anyAllConditions,
		trigger:          trigger,
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
	forEach []kyvernov1.ForEachValidation,
	contextLoader engineapi.EngineContextLoader) *generator {

	g := newGenerator(client, logger, policyContext, policy, rule, contextEntries, anyAllConditions, trigger, contextLoader)

	g.forEach = forEach
	return g
}

func (g *generator) generate() ([]kyvernov1.ResourceSpec, error) {
	responses := []generateResponse{}
	var err error
	var newGenResources []kyvernov1.ResourceSpec

	if err := g.loadContext(context.TODO()); err != nil {
		return newGenResources, fmt.Errorf("failed to load context: %v", err)
	}

	rule, err := variables.SubstituteAllInRule(g.logger, g.policyContext.JSONContext(), g.rule)
	if err != nil {
		g.logger.Error(err, "variable substitution failed for rule", "rule", rule.Name)
		return nil, err
	}

	target := rule.Generation.ResourceSpec
	logger := g.logger.WithValues("target", target.String())

	if rule.Generation.Clone.Name != "" {
		resp := manageClone(logger.WithValues("type", "clone"), target, kyvernov1.ResourceSpec{}, g.policy.GetSpec().UseServerSideApply, rule, g.client)
		responses = append(responses, resp)
	} else if len(rule.Generation.CloneList.Kinds) != 0 {
		responses = manageCloneList(logger.WithValues("type", "cloneList"), target.GetNamespace(), g.policy.GetSpec().UseServerSideApply, rule, g.client)
	} else {
		resp := manageData(logger.WithValues("type", "data"), target, rule.Generation.RawData, rule.Generation.Synchronize, g.client)
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
		common.ManageLabels(newResource, g.trigger, g.policy, rule.Name)
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
				if !rule.Generation.Synchronize {
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
