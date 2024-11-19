package mutation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// resourceInfo contains the Unstructured resource, and if the resource is a subresource, it contains its name and its
// parentResource's group-version-resource
type resourceInfo struct {
	unstructured      unstructured.Unstructured
	subresource       string
	parentResourceGVR metav1.GroupVersionResource
}

type target struct {
	resourceInfo
	context       []kyvernov1.ContextEntry
	preconditions apiextensions.JSON
}

func loadTargets(ctx context.Context, client engineapi.Client, targets []kyvernov1.TargetResourceSpec, policyCtx engineapi.PolicyContext, logger logr.Logger) ([]target, error) {
	var targetObjects []target
	var errors []error
	for i := range targets {
		preconditions := targets[i].GetAnyAllConditions()
		spec, err := resolveSpec(i, targets[i], policyCtx, logger)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		objs, err := getTargets(ctx, client, spec.ResourceSpec, policyCtx, spec.Selector)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		for _, obj := range objs {
			targetObjects = append(targetObjects, target{
				resourceInfo:  obj,
				context:       targets[i].Context,
				preconditions: preconditions,
			})
		}
	}
	return targetObjects, multierr.Combine(errors...)
}

func resolveSpec(i int, target kyvernov1.TargetResourceSpec, ctx engineapi.PolicyContext, logger logr.Logger) (kyvernov1.TargetSelector, error) {
	var s kyvernov1.TargetSelector
	jsonData, err := json.Marshal(target.TargetSelector)
	if err != nil {
		return kyvernov1.TargetSelector{}, fmt.Errorf("failed to marshal the mutation target to JSON: %s", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return kyvernov1.TargetSelector{}, err
	}

	selector, err := variables.SubstituteAll(logger, ctx.JSONContext(), result)
	if err != nil || selector == nil {
		return kyvernov1.TargetSelector{}, fmt.Errorf("failed to substitute variables in target[%d]: %v", i, err)
	}

	substitutedJson, err := json.Marshal(selector)
	if err != nil {
		return kyvernov1.TargetSelector{}, err
	}

	if err := json.Unmarshal(substitutedJson, &s); err != nil {
		return kyvernov1.TargetSelector{}, err
	}

	return s, nil
}

func getTargets(ctx context.Context, client engineapi.Client, target kyvernov1.ResourceSpec, policyCtx engineapi.PolicyContext, lselector *metav1.LabelSelector) ([]resourceInfo, error) {
	namespace := target.Namespace
	name := target.Name
	policy := policyCtx.Policy()
	// if it's namespaced policy, targets has to be loaded only from the policy's namespace
	if policy.IsNamespaced() {
		namespace = policy.GetNamespace()
	}
	group, version, kind, subresource := kubeutils.ParseKindSelector(target.APIVersion + "/" + target.Kind)
	resources, err := client.GetResources(ctx, group, version, kind, subresource, namespace, name, lselector)
	if err != nil {
		return nil, err
	}

	targetObjects := make([]resourceInfo, 0, len(resources))
	for _, resource := range resources {
		targetObjects = append(targetObjects, resourceInfo{
			unstructured: resource.Unstructured,
			subresource:  resource.SubResource,
			parentResourceGVR: metav1.GroupVersionResource{
				Group:    resource.Group,
				Version:  resource.Version,
				Resource: resource.Resource,
			},
		})
	}
	return targetObjects, nil
}

func match(namespacePattern, namePattern, namespace, name string) bool {
	if namespacePattern == "" && namePattern == "" {
		return true
	} else if namespacePattern == "" {
		if wildcard.Match(namePattern, name) {
			return true
		}
	} else if wildcard.Match(namespacePattern, namespace) {
		if namePattern == "" || wildcard.Match(namePattern, name) {
			return true
		}
	}
	return false
}
