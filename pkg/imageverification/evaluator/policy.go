package eval

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/imageverification/match"
	"github.com/kyverno/kyverno/pkg/imageverification/variables"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
)

type EvaluationResult struct {
	Error      error
	Message    string
	Index      int
	Result     bool
	Exceptions []*policiesv1alpha1.CELPolicyException
}

type CompiledPolicy interface {
	Evaluate(context.Context, imagedataloader.ImageContext, admission.Attributes, interface{}, runtime.Object, bool) (*EvaluationResult, error)
}

type compiledPolicy struct {
	failurePolicy   admissionregistrationv1.FailurePolicyType
	matchConditions []cel.Program
	imageRules      []*match.CompiledMatch
	verifications   []policy.CompiledValidation
	imageExtractors []*variables.CompiledImageExtractor
	attestorList    map[string]string
	attestationList map[string]string
	creds           *v1alpha1.Credentials
	exceptions      []policy.CompiledException
}

func (c *compiledPolicy) Evaluate(ctx context.Context, ictx imagedataloader.ImageContext, attr admission.Attributes, request interface{}, namespace runtime.Object, isK8s bool) (*EvaluationResult, error) {
	matched, err := c.match(ctx, attr, request, namespace, c.matchConditions)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, nil
	}

	// check if the resource matches an exception
	if len(c.exceptions) > 0 {
		matchedExceptions := make([]*policiesv1alpha1.CELPolicyException, 0)
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
		data[NamespaceObjectKey] = namespaceVal
		data[RequestKey] = requestVal.Object
		data[ObjectKey] = objectVal
		data[OldObjectKey] = oldObjectVal
	} else {
		data[ObjectKey] = request
	}

	images, err := variables.ExtractImages(c.imageExtractors, data)
	if err != nil {
		return nil, err
	}
	data[ImagesKey] = images
	data[AttestorKey] = c.attestorList
	data[AttestationKey] = c.attestationList

	imgList := []string{}
	for _, v := range images {
		for _, img := range v {
			if apply, err := match.Match(c.imageRules, img); err != nil {
				return nil, err
			} else if apply {
				imgList = append(imgList, img)
			}
		}
	}

	if err := ictx.AddImages(ctx, imgList, imageverify.GetRemoteOptsFromPolicy(c.creds)...); err != nil {
		return nil, err
	}

	for i, v := range c.verifications {
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

			return &EvaluationResult{
				Result:  outcome,
				Message: message,
				Index:   i,
				Error:   err,
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
		data[NamespaceObjectKey] = namespaceVal
		data[RequestKey] = requestVal.Object
		data[ObjectKey] = objectVal
		data[OldObjectKey] = oldObjectVal
	} else {
		data[ObjectKey] = request
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
