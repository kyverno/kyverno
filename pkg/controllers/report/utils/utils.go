package utils

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
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
		for _, rule := range autogen.Default.ComputeRules(policy, "") {
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

func FetchMutatingAdmissionPolicies(mapLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister) ([]admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1alpha1.MutatingAdmissionPolicy
	if pols, err := mapLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchMutatingAdmissionPolicyBindings(mapBindingLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyBindingLister) ([]admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
	var bindings []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
	if pols, err := mapBindingLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			bindings = append(bindings, *pol)
		}
	}
	return bindings, nil
}

func FetchValidatingAdmissionPolicies(vapLister admissionregistrationv1listers.ValidatingAdmissionPolicyLister) ([]admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1.ValidatingAdmissionPolicy
	if pols, err := vapLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchValidatingAdmissionPolicyBindings(vapBindingLister admissionregistrationv1listers.ValidatingAdmissionPolicyBindingLister) ([]admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	var bindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	if pols, err := vapBindingLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			bindings = append(bindings, *pol)
		}
	}
	return bindings, nil
}

func FetchValidatingPolicies(vpolLister policiesv1alpha1listers.ValidatingPolicyLister) ([]policiesv1alpha1.ValidatingPolicy, error) {
	var policies []policiesv1alpha1.ValidatingPolicy
	if pols, err := vpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchMutatingPolicies(mpolLister policiesv1alpha1listers.MutatingPolicyLister) ([]policiesv1alpha1.MutatingPolicy, error) {
	var policies []policiesv1alpha1.MutatingPolicy
	if pols, err := mpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchImageVerificationPolicies(ivpolLister policiesv1alpha1listers.ImageValidatingPolicyLister) ([]policiesv1alpha1.ImageValidatingPolicy, error) {
	var policies []policiesv1alpha1.ImageValidatingPolicy
	if pols, err := ivpolLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchCELPolicyExceptions(celexLister policiesv1alpha1listers.PolicyExceptionLister, namespace string) ([]*policiesv1alpha1.PolicyException, error) {
	exceptions, err := celexLister.PolicyExceptions(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	return exceptions, nil
}
