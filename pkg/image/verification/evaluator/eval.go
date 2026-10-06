package evaluator

import (
	"context"
	"fmt"
	"sync"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type CompiledImageValidatingPolicy struct {
	Policy     policiesv1beta1.ImageValidatingPolicyLike
	Exceptions []*policiesv1beta1.PolicyException
	Actions    sets.Set[admissionregistrationv1.ValidationAction]
}

func Evaluate(ctx context.Context, ivpols []*CompiledImageValidatingPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister corev1listers.SecretLister) (map[string]*EvaluationResult, error) {
	return EvaluateWithTrace(ctx, ivpols, request, admissionAttr, namespace, lister, false)
}

// EvaluateWithTrace is Evaluate with decision tracing optionally turned on, for
// `kyverno apply --explain`. With trace true each result carries its trace, with the policy and
// scope filled in, and a result a match condition skipped is returned (flagged Skipped) instead
// of nil. When a policy errors, the error is returned exactly as without tracing, together with
// the results traced so far, including the failing policy's partial trace, so the caller can
// still show it.
func EvaluateWithTrace(ctx context.Context, ivpols []*CompiledImageValidatingPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister corev1listers.SecretLister, tracing bool) (map[string]*EvaluationResult, error) {
	// leave remote and name options blank, each compiled policy will provide
	// its own credentials or the default global ones.
	ictx, err := imagedataloader.NewImageContext(lister, nil, nil)
	if err != nil {
		return nil, err
	}
	return evaluateWith(ctx, ictx, ivpols, request, admissionAttr, namespace, lister, tracing)
}

// evaluateWith is EvaluateWithTrace with the image context passed in, so tests can supply one
// that never reaches a registry.
func evaluateWith(ctx context.Context, ictx imagedataloader.ImageContext, ivpols []*CompiledImageValidatingPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister corev1listers.SecretLister, tracing bool) (map[string]*EvaluationResult, error) {
	isAdmissionRequest := false
	// nil until proven otherwise: JSON-mode payloads never build a request map.
	var requestMapFn func() (map[string]any, error)
	if r, ok := request.(*admissionv1.AdmissionRequest); ok {
		isAdmissionRequest = true
		// Built at most once for the whole loop below, and lazily: the thunk is
		// only invoked if some policy's Evaluate actually reaches prepareK8sData.
		requestMapFn = sync.OnceValues(func() (map[string]any, error) {
			return compiler.BuildRawRequestMap(r)
		})
	}

	policies := filterPolicies(ivpols, isAdmissionRequest)

	results := make(map[string]*EvaluationResult, len(policies))
	// Shared by every policy evaluated below, so required sees cross-policy evidence.
	verifications := imageverify.NewImageVerificationResults()
	c := NewCompilerWithTrace(lister, tracing)
	compiled := make(map[string]CompiledPolicy, len(policies))
	// there is no matcher on this path; the scope says what was evaluated instead
	scope := trace.ScopeTrace{Applied: true, Reason: "evaluated against a JSON payload, so no matchConstraints apply"}
	if isAdmissionRequest {
		scope.Reason = "evaluated without a matcher, so matchConstraints were not checked here"
	}
	withHeader := func(policy policiesv1beta1.ImageValidatingPolicyLike, result *EvaluationResult) {
		if result == nil || result.Trace == nil {
			return
		}
		result.Trace.PolicyName = policy.GetName()
		result.Trace.PolicyKind = policy.GetKind()
		if result.Trace.PolicyKind == "" {
			result.Trace.PolicyKind = "ImageValidatingPolicy"
		}
		result.Trace.Scope = scope
	}
	for _, ivpol := range policies {
		p, errList := c.Compile(ivpol.Policy, ivpol.Exceptions)
		if errList != nil {
			return nil, fmt.Errorf("failed to compile policy %v", errList)
		}

		result, err := p.Evaluate(ctx, ictx, imageverifycache.DisabledImageVerifyCache(), verifications, admissionAttr, request, namespace, isAdmissionRequest, requestMapFn, nil)
		if err != nil {
			if !tracing {
				return nil, err
			}
			// result is nil here unless tracing is on, in which case it carries the partial trace
			withHeader(ivpol.Policy, result)
			results[ivpol.Policy.GetName()] = result
			return results, err
		}
		withHeader(ivpol.Policy, result)
		results[ivpol.Policy.GetName()] = result
		compiled[ivpol.Policy.GetName()] = p
	}
	// required is settled only now: a policy's images may have been verified by a
	// policy evaluated after it
	for name, result := range results {
		if result == nil || !result.Result {
			continue
		}
		if err := compiled[name].EnforceRequired(result.MatchedImages, verifications); err != nil {
			result.Result = false
			result.Message = err.Error()
			if result.Trace != nil {
				// the validations passed, then required turned the result into a failure; no
				// single expression decided that, so the verdict carries only the message
				result.Trace.Verdict = trace.VerdictTrace{
					Status:  trace.VerdictFail,
					Message: "every validation passed, but validationConfigurations.required failed: " + err.Error(),
				}
			}
		}
	}
	return results, nil
}

func isK8s(request interface{}) bool {
	_, ok := request.(*admissionv1.AdmissionRequest)
	return ok
}

func filterPolicies(ivpols []*CompiledImageValidatingPolicy, isK8s bool) []*CompiledImageValidatingPolicy {
	filteredPolicies := make([]*CompiledImageValidatingPolicy, 0)

	for _, v := range ivpols {
		if v == nil || v.Policy == nil {
			continue
		}
		pol := v.Policy

		if isK8s && pol.GetSpec().EvaluationMode() == policieskyvernoio.EvaluationModeKubernetes {
			filteredPolicies = append(filteredPolicies, v)
		} else if !isK8s && pol.GetSpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
			filteredPolicies = append(filteredPolicies, v)
		}
	}
	return filteredPolicies
}
