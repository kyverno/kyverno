package utils

import (
	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
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
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if cpols, err := cpolLister.List(labels.Everything().Add(*r)); err != nil {
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
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := polLister.Policies(namespace).List(labels.Everything().Add(*r)); err != nil {
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

func FetchMutatingAdmissionPolicies(mapLister admissionregistrationv1beta1listers.MutatingAdmissionPolicyLister) ([]admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1beta1.MutatingAdmissionPolicy
	r, err := getIncludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := mapLister.List(labels.NewSelector().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchMutatingAdmissionPoliciesAlpha(mapLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyLister) ([]admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	var policies []admissionregistrationv1alpha1.MutatingAdmissionPolicy
	r, err := getIncludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := mapLister.List(labels.NewSelector().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchMutatingAdmissionPolicyBindings(mapBindingLister admissionregistrationv1beta1listers.MutatingAdmissionPolicyBindingLister) ([]admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, error) {
	var bindings []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	if pols, err := mapBindingLister.List(labels.Everything()); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			bindings = append(bindings, *pol)
		}
	}
	return bindings, nil
}

func FetchMutatingAdmissionPolicyBindingsAlpha(mapBindingLister admissionregistrationv1alpha1listers.MutatingAdmissionPolicyBindingLister) ([]admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
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
	r, err := getIncludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := vapLister.List(labels.NewSelector().Add(*r)); err != nil {
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
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := vapBindingLister.List(labels.Everything().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			bindings = append(bindings, *pol)
		}
	}
	return bindings, nil
}

func FetchValidatingPolicies(vpolLister policiesv1beta1listers.ValidatingPolicyLister) ([]policiesv1beta1.ValidatingPolicy, error) {
	var policies []policiesv1beta1.ValidatingPolicy
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := vpolLister.List(labels.Everything().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchNamespacedValidatingPolicies(nvpolLister policiesv1beta1listers.NamespacedValidatingPolicyLister, namespace string) ([]policiesv1beta1.NamespacedValidatingPolicy, error) {
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	var pols []*policiesv1beta1.NamespacedValidatingPolicy
	if namespace != "" {
		pols, err = nvpolLister.NamespacedValidatingPolicies(namespace).List(labels.Everything().Add(*r))
	} else {
		pols, err = nvpolLister.List(labels.Everything().Add(*r))
	}
	if err != nil {
		return nil, err
	}
	policies := make([]policiesv1beta1.NamespacedValidatingPolicy, 0, len(pols))
	for _, pol := range pols {
		policies = append(policies, *pol)
	}
	return policies, nil
}

func FetchMutatingPolicies(mpolLister policiesv1beta1listers.MutatingPolicyLister) ([]policiesv1beta1.MutatingPolicy, error) {
	var policies []policiesv1beta1.MutatingPolicy
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := mpolLister.List(labels.Everything().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchNamespacedMutatingPolicies(nmpolLister policiesv1beta1listers.NamespacedMutatingPolicyLister, namespace string) ([]policiesv1beta1.NamespacedMutatingPolicy, error) {
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	var pols []*policiesv1beta1.NamespacedMutatingPolicy
	if namespace != "" {
		pols, err = nmpolLister.NamespacedMutatingPolicies(namespace).List(labels.Everything().Add(*r))
	} else {
		pols, err = nmpolLister.List(labels.Everything().Add(*r))
	}
	if err != nil {
		return nil, err
	}
	policies := make([]policiesv1beta1.NamespacedMutatingPolicy, 0, len(pols))
	for _, pol := range pols {
		policies = append(policies, *pol)
	}
	return policies, nil
}

func FetchImageVerificationPolicies(ivpolLister policiesv1beta1listers.ImageValidatingPolicyLister) ([]policiesv1beta1.ImageValidatingPolicy, error) {
	var policies []policiesv1beta1.ImageValidatingPolicy
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := ivpolLister.List(labels.Everything().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchNamespacedImageVerificationPolicies(nivpolLister policiesv1beta1listers.NamespacedImageValidatingPolicyLister, namespace string) ([]policiesv1beta1.NamespacedImageValidatingPolicy, error) {
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	var pols []*policiesv1beta1.NamespacedImageValidatingPolicy
	if namespace != "" {
		pols, err = nivpolLister.NamespacedImageValidatingPolicies(namespace).List(labels.Everything().Add(*r))
	} else {
		pols, err = nivpolLister.List(labels.Everything().Add(*r))
	}
	if err != nil {
		return nil, err
	}
	policies := make([]policiesv1beta1.NamespacedImageValidatingPolicy, 0, len(pols))
	for _, pol := range pols {
		policies = append(policies, *pol)
	}
	return policies, nil
}

func FetchGeneratingPolicy(gpolLister policiesv1beta1listers.GeneratingPolicyLister) ([]policiesv1beta1.GeneratingPolicy, error) {
	var policies []policiesv1beta1.GeneratingPolicy
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	if pols, err := gpolLister.List(labels.Everything().Add(*r)); err != nil {
		return nil, err
	} else {
		for _, pol := range pols {
			policies = append(policies, *pol)
		}
	}
	return policies, nil
}

func FetchNamespacedGeneratingPolicies(ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister, namespace string) ([]policiesv1beta1.NamespacedGeneratingPolicy, error) {
	r, err := getExcludeReportingLabelRequirement()
	if err != nil {
		return nil, err
	}
	var gpols []*policiesv1beta1.NamespacedGeneratingPolicy
	if namespace != "" {
		gpols, err = ngpolLister.NamespacedGeneratingPolicies(namespace).List(labels.Everything().Add(*r))
	} else {
		gpols, err = ngpolLister.List(labels.Everything().Add(*r))
	}
	if err != nil {
		return nil, err
	}
	policies := make([]policiesv1beta1.NamespacedGeneratingPolicy, 0, len(gpols))
	for _, pol := range gpols {
		policies = append(policies, *pol)
	}
	return policies, nil
}

func FetchCELPolicyExceptions(celexLister policiesv1beta1listers.PolicyExceptionLister, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := celexLister.PolicyExceptions(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	return exceptions, nil
}

func getExcludeReportingLabelRequirement() (*labels.Requirement, error) {
	requirement, err := labels.NewRequirement(
		kyverno.LabelExcludeReporting,
		selection.DoesNotExist,
		nil, // values not needed for DoesNotExist
	)
	if err != nil {
		return nil, err
	}
	return requirement, nil
}

func getIncludeReportingLabelRequirement() (*labels.Requirement, error) {
	requirement, err := labels.NewRequirement(
		kyverno.LabelEnableVAPReporting,
		selection.Equals,
		[]string{"true"},
	)
	if err != nil {
		return nil, err
	}
	return requirement, nil
}
