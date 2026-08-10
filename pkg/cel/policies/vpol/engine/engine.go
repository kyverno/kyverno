package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/cel/autogen/extract"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
)

type (
	EngineRequest  = engine.EngineRequest
	EngineResponse = engine.EngineResponse
	Engine         = engine.Engine[policiesv1beta1.ValidatingPolicyLike]
	Predicate      = func(policiesv1beta1.ValidatingPolicyLike) bool
)

type engineImpl struct {
	provider   Provider
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(provider Provider, nsResolver engine.NamespaceResolver, matcher matching.Matcher) Engine {
	return &engineImpl{
		provider:   provider,
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

func (e *engineImpl) Handle(ctx context.Context, request EngineRequest, predicate Predicate) (EngineResponse, error) {
	var response EngineResponse
	// fetch compiled policies
	policies, err := e.provider.Fetch(ctx)
	if err != nil {
		return response, err
	}
	// if the request is for a json payload, handle it directly
	if request.JsonPayload != nil {
		response.Resource = request.JsonPayload
		for _, policy := range policies {
			response.Policies = append(response.Policies, e.handlePolicy(ctx, policy, request.JsonPayload.Object, nil, nil, nil, request.Context))
		}
		return response, nil
	}
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Resource = &object
	if response.Resource.Object == nil {
		response.Resource = &oldObject
	}
	// default dry run
	dryRun := false
	if request.Request.DryRun != nil {
		dryRun = *request.Request.DryRun
	}
	// create admission attributes
	attr := admission.NewAttributesRecord(
		&object,
		&oldObject,
		schema.GroupVersionKind(request.Request.Kind),
		request.Request.Namespace,
		request.Request.Name,
		schema.GroupVersionResource(request.Request.Resource),
		request.Request.SubResource,
		admission.Operation(request.Request.Operation),
		nil,
		dryRun,
		admissionpolicy.NewUser(request.Request.UserInfo),
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}
	// evaluate policies
	for _, policy := range policies {
		if predicate != nil && !predicate(policy.Policy) {
			continue
		}

		startTime := time.Now()
		pol := e.handlePolicy(ctx, policy, nil, attr, &request.Request, namespace, request.Context)
		for i, rule := range pol.Rules {
			pol.Rules[i] = rule.WithStats(engineapi.NewExecutionStats(startTime, time.Now()))
		}

		response.Policies = append(response.Policies, pol)
	}
	return response, nil
}

func (e *engineImpl) handlePolicy(ctx context.Context, policy Policy, jsonPayload any, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context libs.Context) engine.ValidatingPolicyResponse {
	response := engine.ValidatingPolicyResponse{
		Actions: policy.Actions,
		Policy:  policy.Policy,
	}
	if e.matcher != nil {
		matches, err := e.matchPolicy(policy.CompiledPolicy.MatchConstraints(), attr, namespace)
		if err != nil {
			response.Rules = handlers.WithResponses(engineapi.RuleError("match", engineapi.Validation, "failed to execute matching", err, nil))
			return response
		} else if !matches {
			return response
		}
	}
	var result *compiler.EvaluationResult
	var err error
	switch {
	case jsonPayload != nil:
		result, err = policy.CompiledPolicy.Evaluate(ctx, jsonPayload, nil, nil, nil, context)
	case policy.ExtractionMode:
		result, err = e.evaluateExtracted(ctx, policy, attr, request, namespace, context)
	default:
		result, err = policy.CompiledPolicy.Evaluate(ctx, nil, attr, request, namespace, context)
	}
	// TODO: error is about match conditions here ?
	if err != nil {
		response.Rules = handlers.WithResponses(engineapi.RuleError("evaluation", engineapi.Validation, "failed to load context", err, nil))
	} else if result == nil {
		response.Rules = append(response.Rules, *engineapi.RuleSkip("", engineapi.Validation, "skip", nil).WithSkipReason(engineapi.SkipReasonMatchConditions))
	} else if len(result.Exceptions) > 0 {
		exceptions := make([]engineapi.GenericException, 0, len(result.Exceptions))
		keys := make([]string, 0, len(result.Exceptions))

		var (
			highestPriority int
			selectedIndex   int
		)
		for i, ex := range result.Exceptions {
			key, err := cache.MetaNamespaceKeyFunc(ex)
			if err != nil {
				response.Rules = handlers.WithResponses(
					engineapi.RuleError(
						"exception",
						engineapi.Validation,
						"failed to compute exception key",
						err,
						nil,
					),
				)
				return response
			}

			keys = append(keys, key)
			exceptions = append(exceptions, engineapi.NewCELPolicyException(ex))

			// evaluate exception priority from label
			if val, ok := ex.GetLabels()[reportutils.LabelPolicyExceptionPriority]; ok {
				if p, err := strconv.Atoi(val); err == nil && p > highestPriority {
					highestPriority = p
					selectedIndex = i
				}
			}
		}
		// determine final result based on highest-priority exception
		selectedException := result.Exceptions[selectedIndex]
		reportResult := selectedException.Spec.ReportResult

		joinedKeys := strings.Join(keys, ", ")
		msgPrefix := "rule is %s due to policy exception: " + joinedKeys
		switch reportResult {
		case string(engineapi.RuleStatusPass):
			response.Rules = handlers.WithResponses(
				engineapi.RulePass("exception", engineapi.Validation,
					fmt.Sprintf(msgPrefix, "passed"), nil,
				).WithExceptions(exceptions),
			)
		default:
			response.Rules = handlers.WithResponses(
				engineapi.RuleSkip("exception", engineapi.Validation,
					fmt.Sprintf(msgPrefix, "skipped"), nil,
				).WithExceptions(exceptions),
			)
		}
	} else {
		// TODO: do we want to set a rule name?
		ruleName := ""
		if result.Error != nil {
			response.Rules = append(response.Rules, *engineapi.RuleError(ruleName, engineapi.Validation, "error", result.Error, withValidationIndex(nil, result.Index)))
		} else if result.Result {
			response.Rules = append(response.Rules, *engineapi.RulePass(ruleName, engineapi.Validation, "success", result.AuditAnnotations))
		} else {
			response.Rules = append(response.Rules, *engineapi.RuleFail(ruleName, engineapi.Validation, result.Message, withValidationIndex(result.AuditAnnotations, result.Index)))
		}
	}
	return response
}

// evaluateExtracted implements ExtractionMode: instead of evaluating
// CompiledPolicy against the real admitted object (a custom workload CRD,
// whose shape CompiledPolicy's Pod-targeted rule knows nothing about), it
// extracts every pod-template-shaped subtree, synthesizes a Pod from each,
// and evaluates the same unmodified CompiledPolicy against each synthesized
// Pod in turn. Any failing/erroring template denies the whole request; a
// resource with no discoverable pod template is an explicit error rather
// than a silent pass, since a coverage gap should be visible during this
// phase rather than mistaken for correct enforcement.
func (e *engineImpl) evaluateExtracted(ctx context.Context, policy Policy, attr admission.Attributes, request *admissionv1.AdmissionRequest, namespace runtime.Object, context libs.Context) (*compiler.EvaluationResult, error) {
	obj, ok := attr.GetObject().(*unstructured.Unstructured)
	if !ok || obj == nil {
		return &compiler.EvaluationResult{Error: fmt.Errorf("extraction mode: expected an unstructured object, got %T", attr.GetObject())}, nil
	}
	templates := extract.ExtractPodTemplates(obj.Object)
	if len(templates) == 0 {
		return &compiler.EvaluationResult{Error: fmt.Errorf("extraction mode: no pod template found in %s/%s", obj.GetAPIVersion(), obj.GetKind())}, nil
	}
	var oldByPath map[string]extract.Extracted
	if oldObj, ok := attr.GetOldObject().(*unstructured.Unstructured); ok && oldObj != nil {
		oldTemplates := extract.ExtractPodTemplates(oldObj.Object)
		oldByPath = make(map[string]extract.Extracted, len(oldTemplates))
		for _, t := range oldTemplates {
			oldByPath[t.Path] = t
		}
	}
	var last *compiler.EvaluationResult
	for _, tpl := range templates {
		var oldTpl *extract.Extracted
		if o, ok := oldByPath[tpl.Path]; ok {
			oldTpl = &o
		}
		synthAttr := extract.SynthesizePodAttributes(tpl, oldTpl, attr)
		result, err := policy.CompiledPolicy.Evaluate(ctx, nil, synthAttr, request, namespace, context)
		if err != nil {
			return nil, fmt.Errorf("pod template at %s: %w", tpl.Path, err)
		}
		if result != nil && (result.Error != nil || !result.Result) {
			if result.Message != "" {
				result.Message = fmt.Sprintf("%s (pod template at %s)", result.Message, tpl.Path)
			}
			return result, nil
		}
		last = result
	}
	return last, nil
}

const validationIndexKey = "cel.validationIndex"

// withValidationIndex returns a copy of props with validationIndexKey set to
// the zero-based position of the failing validation expression when that key
// is not already present.
func withValidationIndex(props map[string]string, idx int) map[string]string {
	out := make(map[string]string, len(props)+1)
	for k, v := range props {
		out[k] = v
	}
	if _, exists := out[validationIndexKey]; !exists {
		out[validationIndexKey] = strconv.Itoa(idx)
	}
	return out
}

func (e *engineImpl) matchPolicy(constraints *admissionregistrationv1.MatchResources, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	if constraints == nil {
		return false, nil
	}
	matches, err := e.matcher.Match(&matching.MatchCriteria{Constraints: constraints}, attr, namespace)
	if err != nil {
		return false, err
	}
	return matches, nil
}
