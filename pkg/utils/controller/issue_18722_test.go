package controller

import (
	"context"
	"testing"

	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Repro for #18722: observed (from JSON/informer) has nil map,
// desired (built in Go) has empty non-nil map. They encode to the
// same JSON but reflect.DeepEqual says they differ, so Update fires
// a no-op every reconciliation.
func TestUpdate_NoOpWhenNilVsEmpty(t *testing.T) {
	observed := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Data: nil, // from JSON round-trip
	}
	desired := observed.DeepCopy()
	desired.Data = map[string]string{} // empty, non-nil

	// THE PROOF: this should log false if the bug hypothesis holds.
	t.Logf("reflect.DeepEqual(nil, empty) = %v", datautils.DeepEqual(observed, desired))

	client := fake.NewSimpleClientset(observed)

	updateCalled := false
	client.PrependReactor("update", "configmaps",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			updateCalled = true
			return true, nil, nil
		})

	build := func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	}

	// The second generic type parameter must be the client interface, not the object.
	cmClient := client.CoreV1().ConfigMaps("default")
	_, err := Update[*corev1.ConfigMap, interface {
		Update(context.Context, *corev1.ConfigMap, metav1.UpdateOptions) (*corev1.ConfigMap, error)
	}](context.Background(), observed, cmClient, build)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updateCalled {
		t.Logf("BUG REPRODUCED: Update was called for a semantically-equal object")
	} else {
		t.Logf("OK: no update for semantically-equal object")
	}
}