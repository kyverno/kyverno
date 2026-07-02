package evaluator

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	"github.com/kyverno/sdk/extensions/regcreds"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	var gvr *metav1.GroupVersionResource
	if r, ok := request.(*admissionv1.AdmissionRequest); ok {
		isAdmissionRequest = true
		gvr = requestGVR(r)
	}

	policies := filterPolicies(ivpols, isAdmissionRequest)

	results := make(map[string]*EvaluationResult, len(policies))
	for _, ivpol := range policies {
		allNameOpts := []name.Option{}
		defaultAuthOpts := regcreds.DefaultOpts()
		allAuthOpts := defaultAuthOpts[:]

		if ivpol.Policy.GetSpec().Credentials != nil {
			remoteOpts, nameOpts := regcreds.RemoteOptsFromIvpolCredentials(lister, *ivpol.Policy.GetSpec().Credentials, config.KyvernoNamespace())
			allNameOpts = append(allNameOpts, nameOpts...)
			allAuthOpts = append(allAuthOpts, remoteOpts...)
		}

		ictx, err := imagedataloader.NewImageContext(lister, allAuthOpts, allNameOpts)
		if err != nil {
			return nil, err
		}

		c := NewCompiler(ictx, lister, gvr, imageverifycache.DisabledImageVerifyCache())
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
