package policy

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type CompiledPolicy struct {
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []cel.Program
	auditAnnotations map[string]cel.Program
}

func (p *CompiledPolicy) Evaluate(
	ctx context.Context,
	resource *unstructured.Unstructured,
	namespaceLabels map[string]map[string]string,
) (bool, error) {
	matchConditions := func() (bool, error) {
		var errs []error
		data := map[string]any{
			ObjectKey: resource.UnstructuredContent(),
		}
		for _, matchCondition := range p.matchConditions {
			// evaluate the condition
			out, _, err := matchCondition.ContextEval(ctx, data)
			// check error
			if err != nil {
				errs = append(errs, err)
				continue
			}
			// try to convert to a bool
			result, err := utils.ConvertToNative[bool](out)
			// check error
			if err != nil {
				errs = append(errs, err)
				continue
			}
			// if condition is false, skip
			if !result {
				return false, nil
			}
		}
		return true, multierr.Combine(errs...)
	}
	match, err := matchConditions()
	if err != nil {
		return false, err
	}
	if !match {
		return true, nil
	}
	variables := func() map[string]any {
		vars := lazy.NewMapValue(VariablesType)
		data := map[string]any{
			ObjectKey:    resource.UnstructuredContent(),
			VariablesKey: vars,
		}
		for name, variable := range p.variables {
			vars.Append(name, func(*lazy.MapValue) ref.Val {
				out, _, err := variable.Eval(data)
				if out != nil {
					return out
				}
				if err != nil {
					return types.WrapErr(err)
				}
				return nil
			})
		}
		return data
	}
	data := variables()
	for _, rule := range p.validations {
		out, _, err := rule.Eval(data)
		// check error
		if err != nil {
			return false, err
		}
		response, err := utils.ConvertToNative[bool](out)
		// check error
		if err != nil {
			return false, err
		}
		// if response is false, return
		if !response {
			return false, nil
		}
	}
	return true, nil
}
