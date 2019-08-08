package abool

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestBool(t *testing.T) {
	v := NewBool(true)
	if !v.IsSet() {
		t.Fatal("NewValue(true) failed")
	}

	v = NewBool(false)
	if v.IsSet() {
		t.Fatal("NewValue(false) failed")
	}

	v = New()
	if v.IsSet() {
		t.Fatal("Empty value of AtomicBool should be false")
	}

	v.Set()
	if !v.IsSet() {
		t.Fatal("AtomicBool.Set() failed")
	}

	v.UnSet()
	if v.IsSet() {
		t.Fatal("AtomicBool.UnSet() failed")
	}

	v.SetTo(true)
	if !v.IsSet() {
		t.Fatal("AtomicBool.SetTo(true) failed")
	}

	v.SetTo(false)
	if v.IsSet() {
		t.Fatal("AtomicBool.SetTo(false) failed")
	}

	if set := v.SetToIf(true, false); set || v.IsSet() {
		t.Fatal("AtomicBool.SetTo(true, false) failed")
	}

	if set := v.SetToIf(false, true); !set || !v.IsSet() {
		t.Fatal("AtomicBool.SetTo(false, true) failed")
	}
}

func TestRace(t *testing.T) {
	repeat := 10000
	var wg sync.WaitGroup
	wg.Add(repeat * 3)
	v := New()

	// Writer
	go func() {
		for i := 0; i < repeat; i++ {
			v.Set()
			wg.Done()
		}
	}()

	// Reader
	go func() {
		for i := 0; i < repeat; i++ {
			v.IsSet()
			wg.Done()
		}
	}()

	// Writer
	go func() {
		for i := 0; i < repeat; i++ {
			v.UnSet()
			wg.Done()
		}
	}()
	wg.Wait()
}

func ExampleAtomicBool() {
	cond := New()    // default to false
	cond.Set()       // set to true
	cond.IsSet()     // returns true
	cond.UnSet()     // set to false
	cond.SetTo(true) // set to whatever you want
}

// Benchmark Read

func BenchmarkMutexRead(b *testing.B) {
	var m sync.RWMutex
	var v bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RLock()
		_ = v
		m.RUnlock()
	}
}

func BenchmarkAtomicValueRead(b *testing.B) {
	var v atomic.Value
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.Load() != nil
	}
}

func BenchmarkAtomicBoolRead(b *testing.B) {
	v := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = v.IsSet()
	}
}

// Benchmark Write

func BenchmarkMutexWrite(b *testing.B) {
	var m sync.RWMutex
	var v bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RLock()
		v = true
		m.RUnlock()
	}
	b.StopTimer()
	_ = v
}

func BenchmarkAtomicValueWrite(b *testing.B) {
	var v atomic.Value
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Store(true)
	}
}

func BenchmarkAtomicBoolWrite(b *testing.B) {
	v := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Set()
	}
}

// Benchmark CAS

func BenchmarkMutexCAS(b *testing.B) {
	var m sync.RWMutex
	var v bool
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Lock()
		if !v {
			v = true
		}
		m.Unlock()
	}
	b.StopTimer()
}

func BenchmarkAtomicBoolCAS(b *testing.B) {
	v := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.SetToIf(false, true)
	}
}
