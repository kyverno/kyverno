package compiler

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	deletionPropagationPolicy *metav1.DeletionPropagation
	schedule                  string
	conditions                []cel.Program
	variables                 map[string]cel.Program
	exceptions                []compiler.Exception
}

func (p *Policy) Evaluate(ctx context.Context, object unstructured.Unstructured, context libs.Context) (*EvaluationResult, error) {
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew := map[string]any{
		compiler.GlobalContextKey: globalcontext.Context{ContextInterface: context},
		compiler.HttpKey:          http.Context{ContextInterface: http.NewHTTP(nil)},
		compiler.ImageDataKey:     imagedata.Context{ContextInterface: context},
		compiler.ObjectKey:        object.UnstructuredContent(),
		compiler.ResourceKey:      resource.Context{ContextInterface: context},
		compiler.VariablesKey:     vars,
	}
	for name, variable := range p.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, dataNew)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}

	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1alpha1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions)
			if err != nil {
				return nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
			}
		}
		if len(matchedExceptions) > 0 {
			return &EvaluationResult{Exceptions: matchedExceptions}, nil
		}
	}
	match, err := p.match(ctx, dataNew, p.conditions)
	if err != nil {
		return nil, err
	}

	return &EvaluationResult{Result: match}, nil
}

func (p *Policy) match(ctx context.Context, data map[string]any, conditions []cel.Program) (bool, error) {
	var errs []error
	for _, condition := range conditions {
		// evaluate the condition
		out, _, err := condition.ContextEval(ctx, data)
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
	if err := multierr.Combine(errs...); err == nil {
		return true, nil
	} else {
		return false, err
	}
}
