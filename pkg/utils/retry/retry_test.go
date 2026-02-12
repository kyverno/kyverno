package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

// Mock logger that implements logr.LogSink
type mockLogger struct {
	infoCalls []mockLogCall
}

type mockLogCall struct {
	level       int
	msg         string
	keysAndVals []interface{}
}

func (m *mockLogger) Init(info logr.RuntimeInfo) {}

func (m *mockLogger) Enabled(level int) bool {
	return true
}

func (m *mockLogger) Info(level int, msg string, keysAndValues ...interface{}) {
	m.infoCalls = append(m.infoCalls, mockLogCall{
		level:       level,
		msg:         msg,
		keysAndVals: keysAndValues,
	})
}

func (m *mockLogger) Error(err error, msg string, keysAndValues ...interface{}) {}

func (m *mockLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return m
}

func (m *mockLogger) WithName(name string) logr.LogSink {
	return m
}

// Test: Function succeeds on first try
func TestRetryFunc_SucceedsImmediately(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	callCount := 0

	run := func(ctx context.Context) error {
		callCount++
		return nil // Success on first try
	}

	retryFn := RetryFunc(ctx, 10*time.Millisecond, 1*time.Second, logr.New(logger), "test operation", run)
	err := retryFn()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected function to be called once, got %d calls", callCount)
	}
}

// Test: Function fails then succeeds
func TestRetryFunc_FailsThenSucceeds(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	callCount := 0
	maxRetries := 3

	run := func(ctx context.Context) error {
		callCount++
		if callCount < maxRetries {
			return errors.New("temporary failure")
		}
		return nil // Success on 3rd try
	}

	retryFn := RetryFunc(ctx, 50*time.Millisecond, 1*time.Second, logr.New(logger), "test operation", run)
	err := retryFn()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if callCount != maxRetries {
		t.Fatalf("expected function to be called %d times, got %d calls", maxRetries, callCount)
	}

	// Verify that errors were logged
	if len(logger.infoCalls) < maxRetries-1 {
		t.Fatalf("expected at least %d log calls, got %d", maxRetries-1, len(logger.infoCalls))
	}
}

// Test: Function times out
func TestRetryFunc_TimesOut(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	callCount := 0

	run := func(ctx context.Context) error {
		callCount++
		return errors.New("persistent failure")
	}

	retryFn := RetryFunc(ctx, 50*time.Millisecond, 200*time.Millisecond, logr.New(logger), "test operation", run)
	err := retryFn()

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && err.Error() != "retry times out: persistent failure" {
		t.Fatalf("expected timeout error, got: %v", err)
	}

	if callCount < 2 {
		t.Fatalf("expected multiple retry attempts, got %d", callCount)
	}
}

// Test: Context cancellation
func TestRetryFunc_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := &mockLogger{}
	callCount := 0

	run := func(ctx context.Context) error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second call
		}
		return errors.New("failure")
	}

	retryFn := RetryFunc(ctx, 50*time.Millisecond, 5*time.Second, logr.New(logger), "test operation", run)
	err := retryFn()

	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}

	// Should have been called at least twice
	if callCount < 2 {
		t.Fatalf("expected at least 2 calls, got %d", callCount)
	}
}

// Test: Zero timeout
func TestRetryFunc_ZeroTimeout(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	callCount := 0

	run := func(ctx context.Context) error {
		callCount++
		return errors.New("failure")
	}

	retryFn := RetryFunc(ctx, 10*time.Millisecond, 0, logr.New(logger), "test operation", run)
	err := retryFn()

	if err == nil {
		t.Fatal("expected timeout error with zero timeout, got nil")
	}
}

// Test: Function panics (should propagate panic)
func TestRetryFunc_FunctionPanics(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}

	run := func(ctx context.Context) error {
		panic("unexpected panic")
	}

	retryFn := RetryFunc(ctx, 10*time.Millisecond, 1*time.Second, logr.New(logger), "test operation", run)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate, but it didn't")
		}
	}()

	_ = retryFn()
}

// Test: Retry interval accuracy
func TestRetryFunc_RetryIntervalTiming(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	callCount := 0
	callTimes := []time.Time{}

	run := func(ctx context.Context) error {
		callCount++
		callTimes = append(callTimes, time.Now())
		if callCount < 3 {
			return errors.New("retry")
		}
		return nil
	}

	retryInterval := 100 * time.Millisecond
	retryFn := RetryFunc(ctx, retryInterval, 1*time.Second, logr.New(logger), "test operation", run)
	err := retryFn()

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(callTimes) < 2 {
		t.Fatal("not enough calls to verify timing")
	}

	// Check that intervals are approximately correct (with 50ms tolerance)
	for i := 1; i < len(callTimes); i++ {
		interval := callTimes[i].Sub(callTimes[i-1])
		if interval < retryInterval-50*time.Millisecond || interval > retryInterval+50*time.Millisecond {
			t.Errorf("retry interval %d: expected ~%v, got %v", i, retryInterval, interval)
		}
	}
}

// Test: Logger receives correct message
func TestRetryFunc_LoggerMessage(t *testing.T) {
	ctx := context.Background()
	logger := &mockLogger{}
	expectedMsg := "custom retry message"
	callCount := 0

	run := func(ctx context.Context) error {
		callCount++
		if callCount < 2 {
			return errors.New("test error")
		}
		return nil
	}

	retryFn := RetryFunc(ctx, 10*time.Millisecond, 1*time.Second, logr.New(logger), expectedMsg, run)
	_ = retryFn()

	if len(logger.infoCalls) == 0 {
		t.Fatal("expected log calls, got none")
	}

	if logger.infoCalls[0].msg != expectedMsg {
		t.Errorf("expected log message %q, got %q", expectedMsg, logger.infoCalls[0].msg)
	}
}
