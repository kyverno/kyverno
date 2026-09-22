package policyexception

import (
	"reflect"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLegacyInfos(t *testing.T) {
	exceptions := []*kyvernov2.PolicyException{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "allow-old-app"},
			Spec: kyvernov2.PolicyExceptionSpec{Exceptions: []kyvernov2.Exception{
				{PolicyName: "payments/restrict-images"},
				{PolicyName: "cluster-policy"},
				{PolicyName: "payments/restrict-images"},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "platform", Name: "empty"},
		},
	}

	want := []info{
		{apiGroup: legacyAPIGroup, exceptionNamespace: "payments", exceptionName: "allow-old-app", policyKind: "ClusterPolicy", policyName: "cluster-policy"},
		{apiGroup: legacyAPIGroup, exceptionNamespace: "payments", exceptionName: "allow-old-app", policyKind: "Policy", policyName: "payments/restrict-images"},
		{apiGroup: legacyAPIGroup, exceptionNamespace: "platform", exceptionName: "empty"},
	}
	if got := legacyInfos(exceptions); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacyInfos() = %#v, want %#v", got, want)
	}
}

func TestCelInfos(t *testing.T) {
	exceptions := []*policiesv1beta1.PolicyException{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "payments", Name: "cel-exception"},
			Spec: policiesv1beta1.PolicyExceptionSpec{PolicyRefs: []policiesv1beta1.PolicyRef{
				{Kind: "ValidatingPolicy", Name: "restrict-images"},
				{Kind: "ValidatingPolicy", Name: "restrict-images"},
				{Kind: "MutatingPolicy", Name: "add-label"},
			}},
		},
	}

	want := []info{
		{apiGroup: celAPIGroup, exceptionNamespace: "payments", exceptionName: "cel-exception", policyKind: "MutatingPolicy", policyName: "add-label"},
		{apiGroup: celAPIGroup, exceptionNamespace: "payments", exceptionName: "cel-exception", policyKind: "ValidatingPolicy", policyName: "restrict-images"},
	}
	if got := celInfos(exceptions); !reflect.DeepEqual(got, want) {
		t.Fatalf("celInfos() = %#v, want %#v", got, want)
	}
}

func TestInfosIncludeExpiredResources(t *testing.T) {
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "expired"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt:  &metav1.Time{Time: time.Unix(1, 0)},
			PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: "ValidatingPolicy", Name: "missing-policy"}},
		},
	}

	if got := celInfos([]*policiesv1beta1.PolicyException{exception}); len(got) != 1 || got[0].policyName != "missing-policy" {
		t.Fatalf("celInfos() dropped an inventory reference: %#v", got)
	}
}
