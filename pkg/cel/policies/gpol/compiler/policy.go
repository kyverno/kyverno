package compiler

import (
	"context"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	cellibs "github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/generator"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/http"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type Policy struct {
	matchConditions []cel.Program
	variables       map[string]cel.Program
	generations     []cel.Program
	exceptions      []compiler.Exception
}

func (p *Policy) Evaluate(
	ctx context.Context,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
) ([]*unstructured.Unstructured, []*policiesv1alpha1.PolicyException, error) {
	data, err := prepareData(attr, request, namespace, context)
	if err != nil {
		return nil, nil, err
	}
	allowedImages := make([]string, 0)
	allowedValues := make([]string, 0)
	dataNew := map[string]any{
		compiler.GlobalContextKey:   globalcontext.Context{ContextInterface: data.Context},
		compiler.HttpKey:            http.Context{ContextInterface: http.NewHTTP(nil)},
		compiler.NamespaceObjectKey: data.Namespace,
		compiler.ObjectKey:          data.Object,
		compiler.OldObjectKey:       data.OldObject,
		compiler.RequestKey:         data.Request,
		compiler.ResourceKey:        resource.Context{ContextInterface: data.Context},
		compiler.ImageDataKey:       imagedata.Context{ContextInterface: data.Context},
	}
	// check if the resource matches an exception
	if len(p.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1alpha1.PolicyException, 0)
		for _, polex := range p.exceptions {
			match, err := p.match(ctx, dataNew, polex.MatchConditions)
			if err != nil {
				return nil, nil, err
			}
			if match {
				matchedExceptions = append(matchedExceptions, polex.Exception)
				allowedImages = append(allowedImages, polex.Exception.Spec.Images...)
				allowedValues = append(allowedValues, polex.Exception.Spec.AllowedValues...)
			}
		}
		if len(matchedExceptions) > 0 && len(allowedImages) == 0 && len(allowedValues) == 0 {
			return nil, matchedExceptions, nil
		}
	}
	dataNew[compiler.ExceptionsKey] = cellibs.Exception{
		AllowedImages: allowedImages,
		AllowedValues: allowedValues,
	}
	match, err := p.match(ctx, dataNew, p.matchConditions)
	if err != nil {
		return nil, nil, err
	}
	if !match {
		return nil, nil, nil
	}
	vars := lazy.NewMapValue(compiler.VariablesType)
	dataNew[compiler.VariablesKey] = vars
	dataNew[compiler.GeneratorKey] = generator.Context{ContextInterface: data.Context}
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
	for _, generation := range p.generations {
		_, _, err := generation.ContextEval(ctx, dataNew)
		if err != nil {
			return nil, nil, err
		}
	}

	generatedResources := data.Context.GetGeneratedResources()
	data.Context.ClearGeneratedResources()
	return generatedResources, nil, nil
}

func (p *Policy) match(
	ctx context.Context,
	data map[string]any,
	matchConditions []cel.Program,
) (bool, error) {
	var errs []error
	for _, matchCondition := range matchConditions {
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
	if err := multierr.Combine(errs...); err == nil {
		return true, nil
	} else {
		return false, err
	}
}
