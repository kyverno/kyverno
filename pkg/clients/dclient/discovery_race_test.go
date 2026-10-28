package dclient

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotifyOnChangedRace(t *testing.T) {
	const registrations = 1000

	c := &serverResources{}
	var calls atomic.Int64
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < registrations; i++ {
			c.OnChanged(func() { calls.Add(1) })
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < registrations; i++ {
			c.notify()
		}
	}()
	wg.Wait()

	// All callbacks are registered now; one more notify must invoke every one.
	before := calls.Load()
	c.notify()
	if got := calls.Load() - before; got != registrations {
		t.Fatalf("notify() invoked %d callbacks, want %d", got, registrations)
	}
}

func TestNotifyAllowsReentrantOnChanged(t *testing.T) {
	c := &serverResources{}
	var inner atomic.Int64

	// The outer callback registers another callback while notify() is running.
	c.OnChanged(func() {
		c.OnChanged(func() { inner.Add(1) })
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.notify() // outer runs and registers the inner callback
		c.notify() // snapshot now includes the inner callback, so it runs
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("notify() deadlocked: a callback calling OnChanged() was blocked")
	}

	if got := inner.Load(); got != 1 {
		t.Fatalf("inner callback ran %d times, want 1", got)
	}
}


