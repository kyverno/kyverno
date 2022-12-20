package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"go.uber.org/multierr"
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

func loadTargets(targets []kyvernov1.ResourceSpec, ctx *PolicyContext, logger logr.Logger) ([]resourceInfo, error) {
	var targetObjects []resourceInfo
	var errors []error

	for i := range targets {
		spec, err := resolveSpec(i, targets[i], ctx, logger)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		objs, err := getTargets(spec, ctx)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		targetObjects = append(targetObjects, objs...)
	}

	return targetObjects, multierr.Combine(errors...)
}

func resolveSpec(i int, target kyvernov1.ResourceSpec, ctx *PolicyContext, logger logr.Logger) (kyvernov1.ResourceSpec, error) {
	kind, err := variables.SubstituteAll(logger, ctx.jsonContext, target.Kind)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Kind %s: %v", i, target.Kind, err)
	}

	apiversion, err := variables.SubstituteAll(logger, ctx.jsonContext, target.APIVersion)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].APIVersion %s: %v", i, target.APIVersion, err)
	}

	namespace, err := variables.SubstituteAll(logger, ctx.jsonContext, target.Namespace)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Namespace %s: %v", i, target.Namespace, err)
	}

	name, err := variables.SubstituteAll(logger, ctx.jsonContext, target.Name)
	if err != nil {
		return kyvernov1.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Name %s: %v", i, target.Name, err)
	}

	return kyvernov1.ResourceSpec{
		APIVersion: apiversion.(string),
		Kind:       kind.(string),
		Namespace:  namespace.(string),
		Name:       name.(string),
	}, nil
}

func getTargets(target kyvernov1.ResourceSpec, ctx *PolicyContext) ([]resourceInfo, error) {
	var targetObjects []resourceInfo
	namespace := target.Namespace
	name := target.Name

	// if it's namespaced policy, targets has to be loaded only from the policy's namespace
	if ctx.policy.IsNamespaced() {
		namespace = ctx.policy.GetNamespace()
	}

	apiResource, parentAPIResource, _, err := ctx.client.Discovery().FindResource(target.APIVersion, target.Kind)
	if err != nil {
		return nil, err
	}

	if namespace != "" && name != "" &&
		!wildcard.ContainsWildcard(namespace) && !wildcard.ContainsWildcard(name) {
		// If the target resource is a subresource
		var obj *unstructured.Unstructured
		var parentResourceGVR metav1.GroupVersionResource
		subresourceName := ""
		if kubeutils.IsSubresource(apiResource.Name) {
			apiVersion := metav1.GroupVersion{
				Group:   parentAPIResource.Group,
				Version: parentAPIResource.Version,
			}.String()
			subresourceName = strings.Split(apiResource.Name, "/")[1]
			obj, err = ctx.client.GetResource(context.TODO(), apiVersion, parentAPIResource.Kind, namespace, name, subresourceName)
			parentResourceGVR = metav1.GroupVersionResource{
				Group:    parentAPIResource.Group,
				Version:  parentAPIResource.Version,
				Resource: parentAPIResource.Name,
			}
		} else {
			obj, err = ctx.client.GetResource(context.TODO(), target.APIVersion, target.Kind, namespace, name)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get target %s/%s %s/%s : %v", target.APIVersion, target.Kind, namespace, name, err)
		}

		return []resourceInfo{{unstructured: *obj, subresource: subresourceName, parentResourceGVR: parentResourceGVR}}, nil
	}

	if kubeutils.IsSubresource(apiResource.Name) {
		apiVersion := metav1.GroupVersion{
			Group:   parentAPIResource.Group,
			Version: parentAPIResource.Version,
		}.String()
		objList, err := ctx.client.ListResource(context.TODO(), apiVersion, parentAPIResource.Kind, "", nil)
		if err != nil {
			return nil, err
		}
		var parentObjects []unstructured.Unstructured
		for i := range objList.Items {
			obj := objList.Items[i].DeepCopy()
			if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
				parentObjects = append(parentObjects, *obj)
			}
		}

		for i := range parentObjects {
			parentObj := parentObjects[i]
			subresourceName := strings.Split(apiResource.Name, "/")[1]
			obj, err := ctx.client.GetResource(context.TODO(), parentObj.GetAPIVersion(), parentAPIResource.Kind, parentObj.GetNamespace(), parentObj.GetName(), subresourceName)
			if err != nil {
				return nil, err
			}
			parentResourceGVR := metav1.GroupVersionResource{
				Group:    parentAPIResource.Group,
				Version:  parentAPIResource.Version,
				Resource: parentAPIResource.Name,
			}
			targetObjects = append(targetObjects, resourceInfo{unstructured: *obj, subresource: subresourceName, parentResourceGVR: parentResourceGVR})
		}
	} else {
		// list all targets if wildcard is specified
		objList, err := ctx.client.ListResource(context.TODO(), target.APIVersion, target.Kind, "", nil)
		if err != nil {
			return nil, err
		}

		for i := range objList.Items {
			obj := objList.Items[i].DeepCopy()
			if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
				targetObjects = append(targetObjects, resourceInfo{unstructured: *obj})
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
