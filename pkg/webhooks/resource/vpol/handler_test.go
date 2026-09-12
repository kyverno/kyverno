package vpol

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	fakeclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestAuditCreatesReportForPassingDenyPolicy(t *testing.T) {
	breaker.SetReportsBreaker(breaker.NewBreaker("reports", nil))
	reportutils.NewReportingConfig([]string{"pass"}, "validate")
	client := fakeclient.NewSimpleClientset()
	h := &handler{
		kyvernoClient:    client,
		admissionReports: true,
		eventGen:         event.NewFake(),
	}
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
				"uid":       "resource-uid",
			},
		},
	}
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	}
	request := admissionv1.AdmissionRequest{
		UID:       types.UID("request-uid"),
		Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
		Namespace: "default",
		Name:      "test",
		Operation: admissionv1.Create,
	}
	response := celengine.EngineResponse{
		Resource: resource,
		Policies: []celengine.ValidatingPolicyResponse{{
			Actions: sets.New(admissionregistrationv1.Deny),
			Policy:  policy,
			Rules:   []engineapi.RuleResponse{*engineapi.RulePass("test", engineapi.Validation, "passed", nil)},
		}},
	}

	h.audit(
		context.Background(),
		logr.Discard(),
		handlers.AdmissionRequest{AdmissionRequest: request},
		celengine.EngineRequest{Request: request},
		response,
	)

	reports, err := client.ReportsV1().EphemeralReports("default").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports.Items) != 1 {
		t.Fatalf("expected one admission report for a passing Deny policy, got %d", len(reports.Items))
	}
}
