package event

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// The Event API validates metadata.name as a DNS subdomain (max 253 chars).
// A valid resource name must never produce an invalid Event name.
func TestEventNameFitsAPIServerLimit(t *testing.T) {
	tests := []struct {
		name    string
		nameLen int
	}{
		{"short name", 10},
		{"236 chars (last length that still fits)", 236},
		{"237 chars (overflows with 16-char timestamp)", 237},
		{"253 chars (max valid resource name)", 253},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			var createdNames, invalidNames []string

			// Mimic the API server: reject Events whose name is not a valid DNS subdomain.
			client.PrependReactor("create", "events", func(action k8stesting.Action) (bool, runtime.Object, error) {
				obj := action.(k8stesting.CreateAction).GetObject()
				name := obj.(interface{ GetName() string }).GetName()
				if errs := apimachineryvalidation.NameIsDNSSubdomain(name, false); len(errs) > 0 {
					invalidNames = append(invalidNames, name)
				} else {
					createdNames = append(createdNames, name)
				}
				return false, nil, nil
			})

			gen := NewEventGenerator(client.EventsV1(), logr.Discard(), 1000, config.NewDefaultConfiguration(false))
			defer gen.queue.ShutDown()

			resourceName := strings.Repeat("a", tc.nameLen)
			gen.Add(Info{
				Regarding: corev1.ObjectReference{
					Kind:       "ConfigMap",
					Name:       resourceName,
					Namespace:  "default",
					UID:        "1234",
					APIVersion: "v1",
				},
				Reason:  PolicyViolation,
				Action:  None,
				Message: "test violation",
				Source:  PolicyController,
			})

			// Guard: if Add filtered the event, Get() would block forever.
			if gen.queue.Len() != 1 {
				t.Fatalf("expected 1 queued event, got %d", gen.queue.Len())
			}

			// Run the real worker once. Retries are re-queued with a delay, and
			// the first attempt is all we need to see the name that was sent.
			gen.processNextWorkItem(context.Background())

			if len(invalidNames) > 0 {
				t.Fatalf("generated invalid Event name (len=%d)", len(invalidNames[0]))
			}
			if len(createdNames) != 1 {
				t.Fatalf("expected 1 Event to be created, got %d", len(createdNames))
			}
		})
	}
}