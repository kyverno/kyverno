package utils

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
)

func CanBackgroundProcess(ctx context.Context, p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if p.GetStatus().ValidatingAdmissionPolicy.Generated {
		return false
	}
	if err := policyvalidation.ValidateVariables(ctx, p, true); err != nil {
		return false
	}
	return true
}

func BuildKindSet(ctx context.Context, logger logr.Logger, policies ...kyvernov1.PolicyInterface) sets.Set[string] {
	kinds := sets.New[string]()
	for _, policy := range policies {
		for _, rule := range autogen.Default(ctx).ComputeRules(policy, "") {
			if rule.HasValidate() || rule.HasVerifyImages() {
				kinds.Insert(rule.MatchResources.GetKinds()...)
			}
		}
	}
	return kinds
}

func RemoveNonBackgroundPolicies(ctx context.Context, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var backgroundPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		if CanBackgroundProcess(ctx, pol) {
			backgroundPolicies = append(backgroundPolicies, pol)
		}
	}
	return backgroundPolicies
}

func RemoveNonValidationPolicies(policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var validationPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		spec := pol.GetSpec()
		if spec.HasVerifyImages() || spec.HasValidate() || spec.HasVerifyManifests() {
			validationPolicies = append(validationPolicies, pol)
		}
	}
	return validationPolicies
}

func ReportsAreIdentical(before, after reportsv1.ReportInterface) bool {
	if !datautils.DeepEqual(before.GetAnnotations(), after.GetAnnotations()) {
		return false
	}
	if !datautils.DeepEqual(before.GetLabels(), after.GetLabels()) {
		return false
	}
	b := before.GetResults()
	a := after.GetResults()
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		a := a[i]
		b := b[i]
		a.Timestamp = metav1.Timestamp{}
		b.Timestamp = metav1.Timestamp{}
		if !datautils.DeepEqual(&a, &b) {
			return false
		}
	}
	return true
}

func FetchClusterPolicies(cpolLister kyvernov1listers.ClusterPolicyLister) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if cpols, err := cpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, cpol := range cpols {
			policies = append(policies, cpol)
		}
	}
	return policies, nil
}

func FetchPolicies(polLister kyvernov1listers.PolicyLister, namespace string) ([]kyvernov1.PolicyInterface, error) {
	var policies []kyvernov1.PolicyInterface
	if pols, err := polLister.Policies(namespace).List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, pol)
		}
	}
	return policies, nil
}

func FetchPolicyExceptions(polexLister kyvernov2listers.PolicyExceptionLister, namespace string) ([]kyvernov2.PolicyException, error) {
	var exceptions []kyvernov2.PolicyException
	if polexs, err := polexLister.PolicyExceptions(namespace).List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, polex := range polexs {
			if polex.Spec.BackgroundProcessingEnabled() {
				exceptions = append(exceptions, *polex)
			}
		}
	}
	return exceptions, nil
}

func FetchValidatingAdmissionPolicies(vapLister admissionregistrationv1beta1listers.ValidatingAdmissionPolicyLister) ([]admissionregistrationv1beta1.ValidatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1beta1.ValidatingAdmissionPolicy
	if pols, err := vapLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchValidatingAdmissionPolicyBindings(vapBindingLister admissionregistrationv1beta1listers.ValidatingAdmissionPolicyBindingLister) ([]admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding, error) {
	var bindings []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	if pols, err := vapBindingLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			bindings = append(bindings, *pol)
		}
	}
	return bindings, nil
}
