package ivpol

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	fakeclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestAuditReportsOnlyUnblockedResults(t *testing.T) {
	breaker.SetReportsBreaker(breaker.NewBreaker("reports", nil))
	reportutils.NewReportingConfig([]string{"pass", "fail", "warn", "error", "skip"}, "validate")
	tests := []struct {
		name        string
		actions     sets.Set[admissionregistrationv1.ValidationAction]
		result      *engineapi.RuleResponse
		wantReports int
	}{{
		name:        "passing Deny policy is not blocked",
		actions:     sets.New(admissionregistrationv1.Deny),
		result:      engineapi.RulePass("test", engineapi.ImageVerify, "passed", nil),
		wantReports: 1,
	}, {
		name:        "skipped Deny policy is not blocked",
		actions:     sets.New(admissionregistrationv1.Deny),
		result:      engineapi.RuleSkip("test", engineapi.ImageVerify, "skipped", nil),
		wantReports: 1,
	}, {
		name:        "failed Deny policy is blocked",
		actions:     sets.New(admissionregistrationv1.Deny),
		result:      engineapi.RuleFail("test", engineapi.ImageVerify, "failed", nil),
		wantReports: 0,
	}, {
		name:        "errored Deny policy is blocked",
		actions:     sets.New(admissionregistrationv1.Deny),
		result:      engineapi.RuleError("test", engineapi.ImageVerify, "errored", errors.New("boom"), nil),
		wantReports: 0,
	}, {
		name:        "failed Audit policy is not blocked",
		actions:     sets.New(admissionregistrationv1.Audit),
		result:      engineapi.RuleFail("test", engineapi.ImageVerify, "failed", nil),
		wantReports: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			request := admissionv1.AdmissionRequest{
				UID:       types.UID("request-uid"),
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
				Resource:  metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
				Namespace: "default",
				Name:      "test",
				Operation: admissionv1.Create,
			}
			response := eval.ImageVerifyEngineResponse{
				Resource: resource,
				Policies: []eval.ImageVerifyPolicyResponse{{
					Actions: tt.actions,
					Policy:  &policiesv1beta1.ImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-policy"}},
					Result:  *tt.result,
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
			if len(reports.Items) != tt.wantReports {
				t.Fatalf("expected %d admission reports, got %d", tt.wantReports, len(reports.Items))
			}
		})
	}
}
