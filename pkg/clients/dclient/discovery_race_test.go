package dclient

import (
	"sync"
	"sync/atomic"
	"testing"
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
