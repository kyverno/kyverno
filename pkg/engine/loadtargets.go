package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func loadTargets(logger logr.Logger, targets []*kyverno.TargetMutation, ctx *PolicyContext) ([]unstructured.Unstructured, error) {
	targetObjects := make([]unstructured.Unstructured, len(targets))
	var errors []error

	for i, target := range targets {
		apiversion, err := variables.SubstituteAll(logger, ctx.JSONContext, target.APIVersion)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to substitute variables in target[%d].APIVersion %s: %v", i, target.APIVersion, err))
			continue
		}

		kind, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Kind)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to substitute variables in target[%d].Kind %s: %v", i, target.Kind, err))
			continue
		}

		name, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Name)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to substitute variables in target[%d].Name %s: %v", i, target.Name, err))
			continue
		}

		namespace, err := variables.SubstituteAll(logger, ctx.JSONContext, target.Namespace)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to substitute variables in target[%d].Namespace %s: %v", i, target.Namespace, err))
			continue
		}

		if namespace == "" {
			namespace = "default"
		}

		obj, err := ctx.Client.GetResource(apiversion.(string), kind.(string), namespace.(string), name.(string))
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get target %s/%s %s/%s : %v", apiversion, kind, namespace, name, err))
			continue
		}

		if obj.GetKind() == "" {
			obj.SetKind(kind.(string))
		}

		obj.SetAPIVersion(apiversion.(string))
		targetObjects = append(targetObjects, *obj)
	}

	return targetObjects, validate.CombineErrors(errors)
}
