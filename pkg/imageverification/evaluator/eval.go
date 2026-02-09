package eval

import (
	"context"
	"fmt"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type CompiledImageValidatingPolicy struct {
	Policy     policiesv1beta1.ImageValidatingPolicyLike
	Exceptions []*policiesv1beta1.PolicyException
	Actions    sets.Set[admissionregistrationv1.ValidationAction]
}

func Evaluate(ctx context.Context, ivpols []*CompiledImageValidatingPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister k8scorev1.SecretInterface, registryOpts ...imagedataloader.Option) (map[string]*EvaluationResult, error) {
	ictx, err := imagedataloader.NewImageContext(lister, registryOpts...)
	if err != nil {
		return nil, err
	}

	isAdmissionRequest := false
	var gvr *metav1.GroupVersionResource
	if r, ok := request.(*admissionv1.AdmissionRequest); ok {
		isAdmissionRequest = true
		gvr = requestGVR(r)
	}

	policies := filterPolicies(ivpols, isAdmissionRequest)

	c := NewCompiler(ictx, lister, gvr)
	results := make(map[string]*EvaluationResult, len(policies))
	for _, ivpol := range policies {
		p, errList := c.Compile(ivpol.Policy, ivpol.Exceptions)
		if errList != nil {
			return nil, fmt.Errorf("failed to compile policy %v", errList)
		}

		result, err := p.Evaluate(ctx, ictx, admissionAttr, request, namespace, isAdmissionRequest, nil)
		if err != nil {
			return nil, err
		}
		results[ivpol.Policy.GetName()] = result
	}
	return results, nil
}

func isK8s(request interface{}) bool {
	_, ok := request.(*admissionv1.AdmissionRequest)
	return ok
}

func requestGVR(request *admissionv1.AdmissionRequest) *metav1.GroupVersionResource {
	if request == nil {
		return nil
	}

	return request.RequestResource
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
