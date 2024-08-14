package generate

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type generator struct {
	client  dclient.Interface
	logger  logr.Logger
	policy  kyvernov1.PolicyInterface
	rule    kyvernov1.Rule
	trigger unstructured.Unstructured
}

func newGenerator(client dclient.Interface, logger logr.Logger, policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, trigger unstructured.Unstructured) *generator {
	return &generator{
		client:  client,
		logger:  logger,
		policy:  policy,
		rule:    rule,
		trigger: trigger,
	}
}

func (g *generator) generate() ([]kyvernov1.ResourceSpec, error) {
	responses := []generateResponse{}
	var err error
	var newGenResources []kyvernov1.ResourceSpec

	target := g.rule.Generation.ResourceSpec
	logger := g.logger.WithValues("target", target.String())

	if g.rule.Generation.Clone.Name != "" {
		resp := manageClone(logger.WithValues("type", "clone"), target, kyvernov1.ResourceSpec{}, g.policy, g.rule, g.client)
		responses = append(responses, resp)
	} else if len(g.rule.Generation.CloneList.Kinds) != 0 {
		responses = manageCloneList(logger.WithValues("type", "cloneList"), target.GetNamespace(), g.policy, g.rule, g.client)
	} else {
		resp := manageData(logger.WithValues("type", "data"), target, g.rule.Generation.RawData, g.rule.Generation.Synchronize, g.client)
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
