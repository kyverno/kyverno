package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
)

func CanBackgroundProcess(p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if p.GetStatus().ValidatingAdmissionPolicy.Generated {
		return false
	}
	if err := policyvalidation.ValidateVariables(p, true); err != nil {
		return false
	}
	return true
}

func BuildKindSet(logger logr.Logger, policies ...kyvernov1.PolicyInterface) sets.Set[string] {
	kinds := sets.New[string]()
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			if rule.HasValidate() || rule.HasVerifyImages() {
				kinds.Insert(rule.MatchResources.GetKinds()...)
			}
		}
	}
	return kinds
}

func RemoveNonBackgroundPolicies(policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var backgroundPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		if CanBackgroundProcess(pol) {
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

func ReportsAreIdentical(before, after kyvernov1alpha2.ReportInterface) bool {
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

func FetchValidatingAdmissionPolicies(vapLister admissionregistrationv1alpha1listers.ValidatingAdmissionPolicyLister) ([]admissionregistrationv1alpha1.ValidatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1alpha1.ValidatingAdmissionPolicy
	if pols, err := vapLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}
