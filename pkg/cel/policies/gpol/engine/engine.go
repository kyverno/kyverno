package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

var (
	errNilPolicy         = errors.New("generating policy is nil")
	errNilCompiledPolicy = errors.New("compiled generating policy is nil")
)

type Engine interface {
	Handle(request engine.EngineRequest, policy Policy, cacheRestore bool) (EngineResponse, error)
}

type engineImpl struct {
	nsResolver engine.NamespaceResolver
	matcher    matching.Matcher
}

func NewEngine(nsResolver engine.NamespaceResolver, matcher matching.Matcher) Engine {
	return &engineImpl{
		nsResolver: nsResolver,
		matcher:    matcher,
	}
}

// Handle evaluates a generating policy against the trigger in the provided request.
func (e *engineImpl) Handle(request engine.EngineRequest, policy Policy, cacheRestore bool) (EngineResponse, error) {
	var response EngineResponse
	// load objects
	object, oldObject, err := admissionutils.ExtractResources(nil, request.Request)
	if err != nil {
		return response, err
	}
	response.Trigger = &object
	if response.Trigger.Object == nil {
		response.Trigger = &oldObject
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
		object.GetNamespace(),
		request.Request.Name,
		schema.GroupVersionResource(request.Request.Resource),
		request.Request.SubResource,
		admission.Operation(request.Request.Operation),
		nil,
		dryRun,
		nil,
	)
	// resolve namespace
	var namespace runtime.Object
	if ns := request.Request.Namespace; ns != "" {
		namespace = e.nsResolver(ns)
	}

	startTime := time.Now()
	genReresponse := e.generate(context.TODO(), policy, attr, &request.Request, namespace, request.Context, string(object.GetUID()), cacheRestore)
	if genReresponse.Result != nil {
		genReresponse.Result = ptr.To(genReresponse.Result.WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
	}
	response.Policies = append(
		response.Policies,
		genReresponse,
	)
	return response, nil
}

func (e *engineImpl) generate(
	ctx context.Context,
	policy Policy,
	attr admission.Attributes,
	request *admissionv1.AdmissionRequest,
	namespace runtime.Object,
	context libs.Context,
	triggerUID string,
	cacheRestore bool,
) GeneratingPolicyResponse {
	response := GeneratingPolicyResponse{
		Policy: policy.Policy,
	}
	if policy.Policy == nil {
		response.Result = engineapi.RuleError("", engineapi.Generation, "policy is not provided", errNilPolicy, nil)
		return response
	}
	spec := policy.Policy.GetSpec()
	if e.matcher != nil {
		matches, err := e.matchPolicy(spec.MatchConstraints, attr, namespace)
		if err != nil {
			response.Result = engineapi.RuleError(policy.Policy.GetName(), engineapi.Generation, "failed to execute matching", err, nil)
			return response
		} else if !matches {
			return response
		}
	}
	if policy.CompiledPolicy == nil {
		response.Result = engineapi.RuleError(policy.Policy.GetName(), engineapi.Generation, "policy has not been compiled", errNilCompiledPolicy, nil)
		return response
	}
	context.SetGenerateContext(
		policy.Policy.GetName(),
		request.Name,
		attr.GetNamespace(),
		request.Kind.Version,
		request.Kind.Group,
		request.Kind.Kind,
		triggerUID,
		cacheRestore,
		policy.Policy.GetSpec().UseServerSideApply,
	)
	generatedResources, exceptions, err := policy.CompiledPolicy.Evaluate(ctx, attr, request, namespace, context)
	if err != nil {
		response.Result = engineapi.RuleError(policy.Policy.GetName(), engineapi.Generation, "failed to evaluate policy", err, nil)
		return response
	}
	if len(exceptions) != 0 {
		genericpolex := make([]engineapi.GenericException, 0, len(exceptions))
		keys := make([]string, 0, len(exceptions))

		var (
			highestPriority int
			selectedIndex   int
		)
		for i, ex := range exceptions {
			key, err := cache.MetaNamespaceKeyFunc(ex)
			if err != nil {
				response.Result = engineapi.RuleError(
					"exception",
					engineapi.Generation,
					"failed to compute exception key",
					err,
					nil,
				)
				return response
			}
			keys = append(keys, key)
			genericpolex = append(genericpolex, engineapi.NewCELPolicyException(ex))

			// evaluate exception priority from label
			if val, ok := ex.GetLabels()[reportutils.LabelPolicyExceptionPriority]; ok {
				if p, err := strconv.Atoi(val); err == nil && p > highestPriority {
					highestPriority = p
					selectedIndex = i
				}
			}
		}
		// determine final result based on highest-priority exception
		selectedException := exceptions[selectedIndex]
		reportResult := selectedException.Spec.ReportResult

		joinedKeys := strings.Join(keys, ", ")
		msgPrefix := "rule is %s due to policy exception: " + joinedKeys
		switch reportResult {
		case string(engineapi.RuleStatusPass):
			response.Result = engineapi.RulePass("exception", engineapi.Generation,
				fmt.Sprintf(msgPrefix, "passed"), nil,
			).WithExceptions(genericpolex)
		default:
			response.Result = engineapi.RuleSkip("exception", engineapi.Generation,
				fmt.Sprintf(msgPrefix, "skipped"), nil,
			).WithExceptions(genericpolex)
		}
		return response
	}
	response.Result = engineapi.RulePass(policy.Policy.GetName(), engineapi.Generation, "policy evaluated successfully", nil).WithGeneratedResources(generatedResources)
	return response
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
