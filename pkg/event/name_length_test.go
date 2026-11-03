package event

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var hexRe = regexp.MustCompile(`^[0-9a-f]+$`)
var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// The Event API validates metadata.name as a DNS subdomain (max 253 chars).
// A valid resource name must never produce an invalid Event name.
func TestEventNameFitsAPIServerLimit(t *testing.T) {
	tests := []struct {
		name     string
		nameLen  int
		wantUUID bool
	}{
		{"short name", 10, false},
		{"236 chars (last length that still fits)", 236, false},
		{"237 chars (overflows with 16-char timestamp)", 237, true},
		{"253 chars (max valid resource name)", 253, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()

			var createdNames, invalidNames []string
			var createdEvents []*eventsv1.Event

			// Mimic the API server: reject Events whose name is not a valid DNS subdomain.
			client.PrependReactor("create", "events", func(action k8stesting.Action) (bool, runtime.Object, error) {
				obj := action.(k8stesting.CreateAction).GetObject()
				name := obj.(interface{ GetName() string }).GetName()
				if ev, ok := obj.(*eventsv1.Event); ok {
					createdEvents = append(createdEvents, ev)
				}
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
			const testUID = "1234"
			gen.Add(Info{
				Regarding: corev1.ObjectReference{
					Kind:       "ConfigMap",
					Name:       resourceName,
					Namespace:  "default",
					UID:        testUID,
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
			if len(createdEvents) != 1 {
				t.Fatalf("expected 1 captured Event, got %d", len(createdEvents))
			}
			reg := createdEvents[0].Regarding
			if reg.Name != resourceName {
				t.Errorf("Regarding.Name must stay the original resource name, got %q", reg.Name)
			}
			if reg.UID != testUID {
				t.Errorf("Regarding.UID must stay %q, got %q", testUID, reg.UID)
			}
			got := createdNames[0]
			if tc.wantUUID {
				if !uuidRe.MatchString(got) {
					t.Errorf("expected a UUID fallback name for an overflowing resource, got %q", got)
				}
			} else {
				suffix, ok := strings.CutPrefix(got, resourceName+".")
				if !ok || !hexRe.MatchString(suffix) {
					t.Errorf("expected %q followed by a lowercase hex timestamp, got %q", resourceName+".", got)
				}
			}
		})
	}
}
