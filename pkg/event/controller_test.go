package event

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestEventGenerator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCreated := make(chan struct{})
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("create", "events", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		eventCreated <- struct{}{}
		return true, nil, nil
	})

	logger := logr.Discard()

	eventsClient := clientset.EventsV1()
	eventGenerator := NewEventGenerator(eventsClient, logger, 1000)

	go eventGenerator.Run(ctx, Workers)
	time.Sleep(1 * time.Second)

	info := Info{
		Regarding: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "pod",
			Namespace: "default",
		},
		Reason:  "TestReason",
		Action:  "TestAction",
		Message: "TestMessage",
		Source:  PolicyController,
	}

	eventGenerator.Add(info)

	select {
	case <-eventCreated:
	case <-time.After(wait.ForeverTestTimeout):
		t.Fatal("event not created")
	}
}
