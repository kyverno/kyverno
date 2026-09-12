package ttl

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestManager_getObservedState(t *testing.T) {
	mgr := &manager{
		resController: make(map[schema.GroupVersionResource]stopFunc),
		logger:        logging.WithName(ControllerName),
	}

	gvr := schema.GroupVersionResource{Group: "testgroup", Version: "v1", Resource: "testresources"}
	mgr.resController[gvr] = func() {}

	observed, err := mgr.getObservedState()
	assert.NoError(t, err)
	assert.True(t, observed.Has(gvr))
	assert.Equal(t, 1, observed.Len())
}

func TestManager_Run_Cleanup(t *testing.T) {
	mgr := &manager{
		resController: make(map[schema.GroupVersionResource]stopFunc),
		logger:        logging.WithName(ControllerName),
		interval:      time.Millisecond * 10,
	}

	stopCalled := false
	gvr := schema.GroupVersionResource{Group: "testgroup", Version: "v1", Resource: "testresources"}
	mgr.resController[gvr] = func() {
		stopCalled = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context immediately so Run() exits after checking Done()
	cancel()

	// This should return immediately because ctx is canceled.
	// We want to ensure it locks and executes the deferred cleanup.
	mgr.Run(ctx, 1)

	assert.True(t, stopCalled, "Expected stop function to be called on context cancel")
}
