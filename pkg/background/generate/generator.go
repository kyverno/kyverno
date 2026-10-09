package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/go-logr/logr"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"go.uber.org/multierr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errGeneratedProvenance = errors.New("generated resource provenance failed")

type generator struct {
	client           dclient.Interface
	provenance       *provenance.Store
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

	preconditionsPassed, msg, err := variables.EvaluateConditionsWithContext(g.logger, g.policyContext.JSONContext(), typeConditions, "generate.preconditions")
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
	if err := g.validateCloneSources(pattern); err != nil {
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
			return newGenResources, response.GetError()
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
		g.preserveProvenanceAnnotation(newResource, nil)
		var persisted *unstructured.Unstructured
		if response.GetAction() == Create {
			newResource.SetResourceVersion("")
			if g.policy.GetSpec().UseServerSideApply {
				persisted, err = g.client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
			} else {
				persisted, err = g.client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
			}
			if err != nil {
				// A create collision did not write the existing resource. Retry the
				// rule instead of authenticating another actor's object.
				return newGenResources, err
			}
			if err := g.stampProvenance(context.TODO(), persisted, newResource.GetLabels()); err != nil {
				return newGenResources, err
			}
			if persisted != nil {
				targetMeta.UID = persisted.GetUID()
			}
			logger.V(2).Info("created generate target resource")
			newGenResources = append(newGenResources, targetMeta)
		} else if response.GetAction() == Update {
			generatedObj, err := g.client.GetResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName())
			if err != nil {
				logger.V(2).Info("creating new target due to the failure when fetching", "err", err.Error())
				if g.policy.GetSpec().UseServerSideApply {
					persisted, err = g.client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
				} else {
					persisted, err = g.client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
				}
				if err != nil {
					return newGenResources, err
				}
				if err := g.stampProvenance(context.TODO(), persisted, newResource.GetLabels()); err != nil {
					return newGenResources, err
				}
				if persisted != nil {
					targetMeta.UID = persisted.GetUID()
				}
				newGenResources = append(newGenResources, targetMeta)
			} else {
				effectiveAPIVersion := targetMeta.GetAPIVersion()
				if effectiveAPIVersion == "" {
					effectiveAPIVersion = generatedObj.GetAPIVersion()
					newResource.SetAPIVersion(effectiveAPIVersion)
				}

				effectiveNamespace := targetMeta.GetNamespace()
				if effectiveNamespace == "" && g.isNamespacedResource(effectiveAPIVersion, targetMeta.GetKind()) {
					effectiveNamespace = "default"
				}
				newResource.SetNamespace(effectiveNamespace)
				g.preserveProvenanceAnnotation(newResource, generatedObj)

				if !g.rule.Generation.Synchronize {
					logger.V(4).Info("synchronize disabled, skip syncing changes")
					continue
				}
				if err := validate.MatchPattern(logger, newResource.Object, generatedObj.Object); err == nil {
					if err := validate.MatchPattern(logger, generatedObj.Object, newResource.Object); err == nil {
						if err := g.stampProvenance(context.TODO(), generatedObj, newResource.GetLabels()); err != nil {
							return newGenResources, err
						}
						logger.V(4).Info("patterns match, skipping updates")
						continue
					}
				}

				logger.V(4).Info("updating existing resource")

				if g.policy.GetSpec().UseServerSideApply {
					persisted, err = g.client.ApplyResource(context.TODO(), effectiveAPIVersion, targetMeta.GetKind(), effectiveNamespace, targetMeta.GetName(), newResource, false, "generate")
				} else {
					persisted, err = g.client.UpdateResource(context.TODO(), effectiveAPIVersion, targetMeta.GetKind(), effectiveNamespace, newResource, false)
				}
				if err != nil {
					logger.Error(err, "failed to update resource")
					return newGenResources, err
				}
				if err := g.stampProvenance(context.TODO(), persisted, newResource.GetLabels()); err != nil {
					return newGenResources, err
				}
			}
			logger.V(3).Info("updated generate target resource")
		}
	}
	return newGenResources, nil
}

// Keep the controller-owned stamp out of policy/clone data comparisons. Only an
// existing target's stamp may be retained; source or requested stamps are stale
// for a newly created target. stampProvenance validates or replaces it afterward.
func (g *generator) preserveProvenanceAnnotation(desired, existing *unstructured.Unstructured) {
	if g.provenance == nil {
		return
	}
	annotations := desired.GetAnnotations()
	delete(annotations, provenance.Annotation)
	if existing != nil {
		if stamp := existing.GetAnnotations()[provenance.Annotation]; stamp != "" {
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[provenance.Annotation] = stamp
		}
	}
	if len(annotations) == 0 {
		desired.SetAnnotations(nil)
	} else {
		desired.SetAnnotations(annotations)
	}
}

// stampProvenance only runs after an independently authorized generation rule
// writes a target, or confirms a synchronized target already matches its output.
func (g *generator) stampProvenance(ctx context.Context, persisted *unstructured.Unstructured, expectedLabels map[string]string) error {
	if g.provenance == nil {
		// The offline CLI intentionally has no installation signing key.
		return nil
	}
	if persisted == nil || persisted.GetUID() == "" || persisted.GetResourceVersion() == "" {
		return fmt.Errorf("%w: requires the persisted UID and resourceVersion", errGeneratedProvenance)
	}
	if !maps.Equal(generateRoutingLabels(persisted.GetLabels()), generateRoutingLabels(expectedLabels)) {
		return fmt.Errorf("%w: routing labels changed while applying the generation rule", errGeneratedProvenance)
	}
	stamp, err := g.provenance.Sign(ctx, g.policy, persisted)
	if err != nil {
		return fmt.Errorf("%w: sign generated resource provenance: %w", errGeneratedProvenance, err)
	}
	if persisted.GetAnnotations()[provenance.Annotation] == stamp {
		return nil
	}

	// Both tests must succeed atomically with the annotation write. A replacement
	// object or a concurrent edit must never receive a stamp for this snapshot.
	patch := []map[string]any{
		{"op": "test", "path": "/metadata/uid", "value": persisted.GetUID()},
		{"op": "test", "path": "/metadata/resourceVersion", "value": persisted.GetResourceVersion()},
	}
	if persisted.GetAnnotations() == nil {
		patch = append(patch, map[string]any{"op": "add", "path": "/metadata/annotations", "value": map[string]string{provenance.Annotation: stamp}})
	} else {
		patch = append(patch, map[string]any{"op": "add", "path": "/metadata/annotations/generate.kyverno.io~1provenance", "value": stamp})
	}
	data, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("%w: marshal generated resource provenance patch: %w", errGeneratedProvenance, err)
	}
	if _, err := g.client.PatchResource(ctx, persisted.GetAPIVersion(), persisted.GetKind(), persisted.GetNamespace(), persisted.GetName(), data); err != nil {
		return fmt.Errorf("%w: persist generated resource provenance: %w", errGeneratedProvenance, err)
	}
	return nil
}

func generateRoutingLabels(labels map[string]string) map[string]string {
	routing := make(map[string]string)
	for key, value := range labels {
		if key == kyverno.LabelAppManagedBy || (strings.HasPrefix(key, "generate.kyverno.io/") && key != common.GenerateTypeCloneSourceLabel) {
			routing[key] = value
		}
	}
	return routing
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

		child := newGenerator(g.client,
			g.logger,
			policyContext,
			g.policy,
			g.rule,
			foreach.Context,
			foreach.AnyAllConditions,
			g.trigger,
			foreach.GeneratePattern,
			g.contextLoader)
		child.provenance = g.provenance
		gen, err := child.generate()
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

func (g *generator) isNamespacedResource(apiVersion, kind string) bool {
	if apiVersion == "" || kind == "" {
		return true
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		g.logger.V(4).Info("failed to parse apiVersion for generated resource scope lookup", "apiVersion", apiVersion, "kind", kind, "error", err.Error())
		return true
	}
	resources, err := g.client.Discovery().FindResources(gv.Group, gv.Version, kind, "")
	if err != nil {
		g.logger.V(4).Info("failed to discover generated resource scope", "apiVersion", apiVersion, "kind", kind, "error", err.Error())
		return true
	}
	for _, resource := range resources {
		return resource.Namespaced
	}
	return true
}
