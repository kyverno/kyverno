package dclient

import (
	"sync"
	"testing"
)

func TestNotifyOnChangedRace(t *testing.T) {
	c := &serverResources{}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			c.OnChanged(func() {})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			c.notify()
		}
	}()
	wg.Wait()
}
