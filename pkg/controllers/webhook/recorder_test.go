package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRecordDoesNotBlockOtherMethods verifies that Ready and Reset do not
// deadlock when Record is blocking on channel send. This is a regression test
// for a bug where Record held the mutex while doing a blocking channel send.
func TestRecordDoesNotBlockOtherMethods(t *testing.T) {
	notifyChan := make(chan string)
	recorder := NewStateRecorder(notifyChan).(*Recorder)

	// Start Record in goroutine - it will block on channel send
	go recorder.Record("test-key")

	// Give Record time to acquire lock and block on channel send
	time.Sleep(50 * time.Millisecond)

	// Ready should not deadlock even though Record is blocking on send
	done := make(chan struct{})
	go func() {
		recorder.Ready("test-key")
		close(done)
	}()

	select {
	case <-done:
		// Ready returned without deadlocking
	case <-time.After(1 * time.Second):
		t.Fatal("Ready deadlocked while Record was blocking on channel send")
	}

	// Drain channel to unblock Record
	<-recorder.NotifyChannel()

	// Verify data was recorded
	ready, ok := recorder.Ready("test-key")
	assert.True(t, ok, "key should exist in data map")
	assert.True(t, ready, "key should be marked as ready")
}
