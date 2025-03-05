package eval

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func Evaluate(ctx context.Context, logger logr.Logger, ivpols []*v1alpha1.ImageVerificationPolicy, request interface{}, admissionAttr admission.Attributes, namespace runtime.Object, lister k8scorev1.SecretInterface, registryOpts ...imagedataloader.Option) ([]*EvaluationResult, error) {
	ictx, err := imagedataloader.NewImageContext(lister, registryOpts...)
	if err != nil {
		return nil, err
	}

	// TODO: use environmentconfig, add support for other controllers (autogen)
	isAdmissionRequest := false
	var gvr *metav1.GroupVersionResource
	if r, ok := request.(*admissionv1.AdmissionRequest); ok {
		isAdmissionRequest = true
		gvr = requestGVR(r)
	}

	policies := filterPolicies(ivpols, isAdmissionRequest)

	c := NewCompiler(ictx, lister, gvr)
	results := make([]*EvaluationResult, 0)
	for _, ivpol := range policies {
		p, errList := c.Compile(logger, ivpol)
		if errList != nil {
			return nil, fmt.Errorf("failed to compile policy %v", errList)
		}

		result, err := p.Evaluate(ctx, ictx, admissionAttr, request, namespace, isAdmissionRequest)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
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

func filterPolicies(ivpols []*v1alpha1.ImageVerificationPolicy, isK8s bool) []*v1alpha1.ImageVerificationPolicy {
	filteredPolicies := make([]*v1alpha1.ImageVerificationPolicy, 0)

	for _, v := range ivpols {
		if v == nil {
			continue
		}

		if isK8s && v.Spec.EvaluationMode() == v1alpha1.EvaluationModeKubernetes {
			filteredPolicies = append(filteredPolicies, v)
		} else if !isK8s && v.Spec.EvaluationMode() == v1alpha1.EvaluationModeJSON {
			filteredPolicies = append(filteredPolicies, v)
		}
	}
	return filteredPolicies
}
