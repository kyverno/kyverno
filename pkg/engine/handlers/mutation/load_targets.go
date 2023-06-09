package mutation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
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

func loadTargets(client dclient.Interface, targets []kyvernov1.TargetResourceSpec, ctx engineapi.PolicyContext, logger logr.Logger) ([]target, error) {
	var targetObjects []target
	var errors []error
	for i := range targets {
		preconditions := targets[i].GetAnyAllConditions()
		spec, err := resolveSpec(i, targets[i], ctx, logger)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		objs, err := getTargets(client, spec, ctx)
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

func resolveSpec(i int, target kyvernov1.TargetResourceSpec, ctx engineapi.PolicyContext, logger logr.Logger) (kyvernov1.ResourceSpec, error) {
	kind, err := variables.SubstituteAll(logger, ctx.JSONContext(), target.Kind)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Kind %s, value: %v, err: %v", i, target.Kind, kind, err)
	}
	apiversion, err := variables.SubstituteAll(logger, ctx.JSONContext(), target.APIVersion)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].APIVersion %s, value: %v, err: %v", i, target.APIVersion, apiversion, err)
	}
	namespace, err := variables.SubstituteAll(logger, ctx.JSONContext(), target.Namespace)
	if err != nil || namespace == nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Namespace %s, value: %v, err: %v", i, target.Namespace, namespace, err)
	}
	name, err := variables.SubstituteAll(logger, ctx.JSONContext(), target.Name)
	if err != nil || name == nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Name %s, value: %v, err: %v", i, target.Name, name, err)
	}
	return kyvernov1.ResourceSpec{
		APIVersion: apiversion.(string),
		Kind:       kind.(string),
		Namespace:  namespace.(string),
		Name:       name.(string),
	}, nil
}

func getTargets(client dclient.Interface, target kyvernov1.ResourceSpec, ctx engineapi.PolicyContext) ([]resourceInfo, error) {
	var targetObjects []resourceInfo
	namespace := target.Namespace
	name := target.Name
	policy := ctx.Policy()
	// if it's namespaced policy, targets has to be loaded only from the policy's namespace
	if policy.IsNamespaced() {
		namespace = policy.GetNamespace()
	}
	group, version, kind, subresource := kubeutils.ParseKindSelector(target.APIVersion + "/" + target.Kind)
	gvrss, err := client.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	for gvrs := range gvrss {
		dyn := client.GetDynamicInterface().Resource(gvrs.GroupVersionResource())
		var sub []string
		if gvrs.SubResource != "" {
			sub = []string{gvrs.SubResource}
		}
		// we can use `GET` directly
		if namespace != "" && name != "" && !wildcard.ContainsWildcard(namespace) && !wildcard.ContainsWildcard(name) {
			var obj *unstructured.Unstructured
			var err error
			obj, err = dyn.Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{}, sub...)
			if err != nil {
				return nil, err
			}
			targetObjects = append(targetObjects, resourceInfo{
				unstructured:      *obj,
				subresource:       gvrs.SubResource,
				parentResourceGVR: metav1.GroupVersionResource(gvrs.GroupVersionResource()),
			})
		} else {
			// we can use `LIST`
			if gvrs.SubResource == "" {
				list, err := dyn.List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, obj := range list.Items {
					if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
						targetObjects = append(targetObjects, resourceInfo{unstructured: obj})
					}
				}
			} else {
				// we need to use `LIST` / `GET`
				list, err := dyn.List(context.TODO(), metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				var parentObjects []unstructured.Unstructured
				for _, obj := range list.Items {
					if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
						parentObjects = append(parentObjects, obj)
					}
				}
				for _, parentObject := range parentObjects {
					var obj *unstructured.Unstructured
					var err error
					if parentObject.GetNamespace() == "" {
						obj, err = dyn.Get(context.TODO(), name, metav1.GetOptions{}, sub...)
					} else {
						obj, err = dyn.Namespace(parentObject.GetNamespace()).Get(context.TODO(), name, metav1.GetOptions{}, sub...)
					}
					if err != nil {
						return nil, err
					}
					targetObjects = append(targetObjects, resourceInfo{
						unstructured:      *obj,
						subresource:       gvrs.SubResource,
						parentResourceGVR: metav1.GroupVersionResource(gvrs.GroupVersionResource()),
					})
				}
			}
		}
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
