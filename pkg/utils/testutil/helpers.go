package testutil

import (
	"testing"
	"time"
)

// MustNotError fails the test if err is not nil
func MustNotError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// Eventually retries a condition until it succeeds or times out
func Eventually(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		<-ticker.C
	}
	t.Fatalf("timeout: %s", msg)
}
