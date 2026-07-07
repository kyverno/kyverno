package main

import (
	"context"
	"errors"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestDeleteWebhooksDeletesMatchingWebhookConfigurations(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "mutating-match",
				Labels: map[string]string{"webhook.kyverno.io/managed-by": "kyverno"},
			},
		},
		&admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "validating-match",
				Labels: map[string]string{"webhook.kyverno.io/managed-by": "kyverno"},
			},
		},
		&admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "validating-keep",
				Labels: map[string]string{"webhook.kyverno.io/managed-by": "other"},
			},
		},
	)

	err := deleteWebhooks(context.Background(), client, "webhook.kyverno.io/managed-by=kyverno")
	if err != nil {
		t.Fatalf("deleteWebhooks() error = %v", err)
	}

	mwCfgs, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing mutating webhook configurations: %v", err)
	}
	if len(mwCfgs.Items) != 0 {
		t.Fatalf("expected mutating webhook configurations to be deleted, got %d", len(mwCfgs.Items))
	}

	vwCfgs, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing validating webhook configurations: %v", err)
	}
	if len(vwCfgs.Items) != 1 {
		t.Fatalf("expected one validating webhook configuration to remain, got %d", len(vwCfgs.Items))
	}
	if got := vwCfgs.Items[0].Name; got != "validating-keep" {
		t.Fatalf("expected validating-keep to remain, got %s", got)
	}
}

func TestDeleteWebhooksReturnsDeleteErrors(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset(
		&admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "validating-match",
				Labels: map[string]string{"webhook.kyverno.io/managed-by": "kyverno"},
			},
		},
	)
	client.PrependReactor("delete", "validatingwebhookconfigurations", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("delete failed")
	})

	err := deleteWebhooks(context.Background(), client, "webhook.kyverno.io/managed-by=kyverno")
	if err == nil {
		t.Fatal("expected deleteWebhooks() to return an error")
	}
	if got := err.Error(); got != "deleting validating webhook configuration validating-match: delete failed" {
		t.Fatalf("unexpected error: %s", got)
	}
}
