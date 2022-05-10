package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/go-wildcard"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	engineUtils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	stringutils "github.com/kyverno/kyverno/pkg/utils/string"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func loadTargets(targets []kyverno.ResourceSpec, ctx *PolicyContext, logger logr.Logger) ([]unstructured.Unstructured, error) {
	targetObjects := []unstructured.Unstructured{}
	var errors []error

	for i := range targets {
		spec, err := resolveSpec(i, targets[i], ctx, logger)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		objs, err := getTargets(spec, ctx, logger)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		targetObjects = append(targetObjects, objs...)
	}

	return targetObjects, engineUtils.CombineErrors(errors)
}

func resolveSpec(i int, target kyverno.ResourceSpec, ctx *PolicyContext, logger logr.Logger) (kyverno.ResourceSpec, error) {
	kind, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Kind)
	if err != nil {
		return kyverno.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Kind %s: %v", i, target.Kind, err)
	}

	apiversion, err := variables.SubstituteAll(logger, ctx.JSONContext, target.APIVersion)
	if err != nil {
		return kyverno.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].APIVersion %s: %v", i, target.APIVersion, err)
	}

	namespace, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Namespace)
	if err != nil {
		return kyverno.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Namespace %s: %v", i, target.Namespace, err)
	}

	name, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Name)
	if err != nil {
		return kyverno.ResourceSpec{}, fmt.Errorf("failed to substitute variables in target[%d].Name %s: %v", i, target.Name, err)
	}

	return kyverno.ResourceSpec{
		APIVersion: apiversion.(string),
		Kind:       kind.(string),
		Namespace:  namespace.(string),
		Name:       name.(string),
	}, nil
}

func getTargets(target kyverno.ResourceSpec, ctx *PolicyContext, logger logr.Logger) ([]unstructured.Unstructured, error) {
	var targetObjects []unstructured.Unstructured
	namespace := target.Namespace
	name := target.Name

	if namespace != "" && name != "" &&
		!stringutils.ContainsWildcard(namespace) && !stringutils.ContainsWildcard(name) {
		obj, err := ctx.Client.GetResource(target.APIVersion, target.Kind, namespace, name)
		if err != nil {
			return nil, fmt.Errorf("failed to get target %s/%s %s/%s : %v", target.APIVersion, target.Kind, namespace, name, err)
		}
		return []unstructured.Unstructured{*obj}, nil
	}

	// list all targets if wildcard is specified
	objList, err := ctx.Client.ListResource(target.APIVersion, target.Kind, "", nil)
	if err != nil {
		return nil, err
	}

	for i := range objList.Items {
		obj := objList.Items[i].DeepCopy()
		if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
			targetObjects = append(targetObjects, *obj)
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
