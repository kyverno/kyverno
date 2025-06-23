package eval

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	engine "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/libs/globalcontext"
	"github.com/kyverno/kyverno/pkg/cel/libs/imagedata"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/libs/resource"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/variables"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/cel/lazy"
)

type EvaluationResult struct {
	Error            error
	Message          string
	Index            int
	Result           bool
	AuditAnnotations map[string]string
	Exceptions       []*policiesv1alpha1.PolicyException
}

type CompiledPolicy interface {
	Evaluate(context.Context, imagedataloader.ImageContext, admission.Attributes, interface{}, runtime.Object, bool, libs.Context) (*EvaluationResult, error)
}

type compiledPolicy struct {
	failurePolicy        admissionregistrationv1.FailurePolicyType
	matchConditions      []cel.Program
	matchImageReferences []engine.MatchImageReference
	validations          []engine.Validation
	imageExtractors      map[string]engine.ImageExtractor
	attestors            []*variables.CompiledAttestor
	attestationList      map[string]string
	auditAnnotations     map[string]cel.Program
	creds                *v1alpha1.Credentials
	exceptions           []engine.Exception
	variables            map[string]cel.Program
}

func (c *compiledPolicy) Evaluate(ctx context.Context, ictx imagedataloader.ImageContext, attr admission.Attributes, request interface{}, namespace runtime.Object, isK8s bool, context libs.Context) (*EvaluationResult, error) {
	matched, err := c.match(ctx, attr, request, namespace, c.matchConditions)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}
	// check if the resource matches an exception
	if len(c.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1alpha1.PolicyException, 0)
		for _, polex := range c.exceptions {
			match, err := c.match(ctx, attr, request, namespace, polex.MatchConditions)
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
	data := map[string]any{}
	vars := lazy.NewMapValue(engine.VariablesType)
	for name, variable := range c.variables {
		vars.Append(name, func(*lazy.MapValue) ref.Val {
			out, _, err := variable.ContextEval(ctx, data)
			if out != nil {
				return out
			}
			if err != nil {
				return types.WrapErr(err)
			}
			return nil
		})
	}
	if isK8s {
		namespaceVal, err := objectToResolveVal(namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
		}
		requestVal, err := convertObjectToUnstructured(request)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
		objectVal, err := objectToResolveVal(attr.GetObject())
		if err != nil {
			return nil, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
		}
		oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
		if err != nil {
			return nil, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
		}
		data[engine.NamespaceObjectKey] = namespaceVal
		data[engine.RequestKey] = requestVal.Object
		data[engine.ObjectKey] = objectVal
		data[engine.OldObjectKey] = oldObjectVal
		data[engine.VariablesKey] = vars
		data[engine.GlobalContextKey] = globalcontext.Context{ContextInterface: context}
		data[engine.ImageDataKey] = imagedata.Context{ContextInterface: context}
		data[engine.ResourceKey] = resource.Context{ContextInterface: context}
	} else {
		data[engine.ObjectKey] = request
	}
	images, err := engine.ExtractImages(data, c.imageExtractors)
	if err != nil {
		return nil, err
	}
	data[engine.ImagesKey] = images
	data[engine.AttestationsKey] = c.attestationList
	attestors := lazy.NewMapValue(cel.DynType)
	for _, attestor := range c.attestors {
		attestors.Append(attestor.Key, func(*lazy.MapValue) ref.Val {
			data, err := attestor.Evaluate(data)
			if err != nil {
				return types.WrapErr(err)
			}
			return data
		})
	}
	data[engine.AttestorsKey] = attestors

	imgList := []string{}
	for _, v := range images {
		for _, img := range v {
			if apply, err := matching.MatchImage(img, c.matchImageReferences...); err != nil {
				return nil, err
			} else if apply {
				imgList = append(imgList, img)
			}
		}
	}
	if err := ictx.AddImages(ctx, imgList, imageverify.GetRemoteOptsFromPolicy(c.creds)...); err != nil {
		return nil, err
	}
	for i, v := range c.validations {
		out, _, err := v.Program.ContextEval(ctx, data)
		if err != nil {
			return nil, err
		}
		// evaluate only when rule fails
		if outcome, err := utils.ConvertToNative[bool](out); err == nil && !outcome {
			message := v.Message
			if v.MessageExpression != nil {
				if out, _, err := v.MessageExpression.ContextEval(ctx, data); err != nil {
					message = fmt.Sprintf("failed to evaluate message expression: %s", err)
				} else if msg, err := utils.ConvertToNative[string](out); err != nil {
					message = fmt.Sprintf("failed to convert message expression to string: %s", err)
				} else {
					message = msg
				}
			}
			auditAnnotations := make(map[string]string, 0)
			for key, annotation := range c.auditAnnotations {
				out, _, err := annotation.ContextEval(ctx, data)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate auditAnnotation '%s': %w", key, err)
				}
				// evaluate only when rule fails
				if outcome, err := utils.ConvertToNative[string](out); err == nil && outcome != "" {
					auditAnnotations[key] = outcome
				} else if err != nil {
					return nil, fmt.Errorf("failed to convert auditAnnotation '%s' expression: %w", key, err)
				}
			}
			return &EvaluationResult{
				Result:           outcome,
				Message:          message,
				AuditAnnotations: auditAnnotations,
				Index:            i,
				Error:            err,
			}, nil
		} else if err != nil {
			return &EvaluationResult{Error: err}, nil
		}
	}
	return &EvaluationResult{Result: true}, nil
}

func (p *compiledPolicy) match(
	ctx context.Context,
	attr admission.Attributes,
	request interface{},
	namespace runtime.Object,
	matchConditions []cel.Program,
) (bool, error) {
	data := make(map[string]any)
	if isK8s(request) {
		namespaceVal, err := objectToResolveVal(namespace)
		if err != nil {
			return false, fmt.Errorf("failed to prepare namespace variable for evaluation: %w", err)
		}
		requestVal, err := convertObjectToUnstructured(request)
		if err != nil {
			return false, fmt.Errorf("failed to prepare request variable for evaluation: %w", err)
		}
		objectVal, err := objectToResolveVal(attr.GetObject())
		if err != nil {
			return false, fmt.Errorf("failed to prepare object variable for evaluation: %w", err)
		}
		oldObjectVal, err := objectToResolveVal(attr.GetOldObject())
		if err != nil {
			return false, fmt.Errorf("failed to prepare oldObject variable for evaluation: %w", err)
		}
		data[engine.NamespaceObjectKey] = namespaceVal
		data[engine.RequestKey] = requestVal.Object
		data[engine.ObjectKey] = objectVal
		data[engine.OldObjectKey] = oldObjectVal
	} else {
		data[engine.ObjectKey] = request
	}
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
	} else if p.failurePolicy == admissionregistrationv1.Ignore {
		return false, nil
	} else {
		return false, err
	}
}

func convertObjectToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return &unstructured.Unstructured{Object: nil}, nil
	}
	ret, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: ret}, nil
}

func objectToResolveVal(r runtime.Object) (interface{}, error) {
	if r == nil || reflect.ValueOf(r).IsNil() {
		return nil, nil
	}
	v, err := convertObjectToUnstructured(r)
	if err != nil {
		return nil, err
	}
	return v.Object, nil
}
