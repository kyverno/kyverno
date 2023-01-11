package utils

import (
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/policy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func CanBackgroundProcess(p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if err := policy.ValidateVariables(p, true); err != nil {
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
		if spec.HasVerifyImages() || spec.HasValidate() || spec.HasYAMLSignatureVerify() {
			validationPolicies = append(validationPolicies, pol)
		}
	}
	return validationPolicies
}

func ReportsAreIdentical(before, after kyvernov1alpha2.ReportInterface) bool {
	if !reflect.DeepEqual(before.GetAnnotations(), after.GetAnnotations()) {
		return false
	}
	if !reflect.DeepEqual(before.GetLabels(), after.GetLabels()) {
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
		if !reflect.DeepEqual(&a, &b) {
			return false
		}
	}
	return true
}
