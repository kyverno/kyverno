package framework

import (
	"github.com/google/uuid"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// MockEventGen captures events for test assertions.
type MockEventGen struct {
	Events []event.Info
}

func (m *MockEventGen) Add(infoList ...event.Info) {
	m.Events = append(m.Events, infoList...)
}

// PodAdmissionRequest builds a handlers.AdmissionRequest for a Pod CREATE operation.
func PodAdmissionRequest(name, namespace string, raw []byte) handlers.AdmissionRequest {
	return handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID(uuid.New().String()),
			Operation: admissionv1.Create,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Name:      name,
			Namespace: namespace,
			Object:    runtime.RawExtension{Raw: raw},
			UserInfo:  authenticationv1.UserInfo{Username: "test-user"},
		},
	}
}

// PodMatchRules returns MatchResources that match pods on CREATE operations.
func PodMatchRules() *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
	}
}
