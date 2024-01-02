package event

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGeneratorWithLargeBatch(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	logger := logr.Discard()

	gen := NewEventGenerator(fakeClient.EventsV1(), logger)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		gen.Run(ctx, Workers, &wg)
	}()

	for i := 0; i < 1000; i++ {
		event := Info{
			Regarding: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      fmt.Sprintf("pod-%d", i),
				Namespace: "default",
			},
			Reason:  "TestReason",
			Action:  "TestAction",
			Message: fmt.Sprintf("TestMessage-%d", i),
			Source:  PolicyController,
		}
		gen.Add(event)
	}

	lastEvent := Info{
		Regarding: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "pod-1000",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Action:  "TestAction",
		Message: "TestMessage-1000",
		Source:  PolicyController,
	}

	// Add the 1001st event (this should fail)
	gen.Add(lastEvent)

	// TODO: Check for emitted events using the fakeClient

	cancel()
	wg.Wait()

	// TODO: Assert event created or not
}
