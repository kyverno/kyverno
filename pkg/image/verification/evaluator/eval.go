package evaluator

import (
	"context"
	"fmt"
	"sync"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs/imageverify"
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
	// leave remote and name options blank, each compiled policy will provide
	// its own credentials or the default global ones.
	ictx, err := imagedataloader.NewImageContext(lister, nil, nil)
	if err != nil {
		return nil, err
	}

	results := make(map[string]*EvaluationResult, len(policies))
	// Shared by every policy evaluated below, so required sees cross-policy evidence.
	verifications := imageverify.NewImageVerificationResults()
	c := NewCompiler(lister)
	compiled := make(map[string]CompiledPolicy, len(policies))
	for _, ivpol := range policies {
		p, errList := c.Compile(ivpol.Policy, ivpol.Exceptions)
		if errList != nil {
			return nil, fmt.Errorf("failed to compile policy %v", errList)
		}

		result, err := p.Evaluate(ctx, &imageverify.Runtime{ImageContext: ictx, Cache: imageverifycache.DisabledImageVerifyCache(), Results: verifications}, admissionAttr, request, namespace, isAdmissionRequest, requestMapFn, nil)
		if err != nil {
			return nil, err
		}
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
