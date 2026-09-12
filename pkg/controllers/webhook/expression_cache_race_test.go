package webhook

// Regression test for https://github.com/kyverno/kyverno/issues/17463
//
// Run with: go test -race -count=1 -run TestGetOrCompile_RaceWithAddExpression ./...
//
// Before the fix: fatal error: concurrent map read and map write (detected by -race
// or the Go runtime on maps).
// After the fix:  test passes cleanly under -race.

import (
	"sync"
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func makeCondition(name, expr string) admissionregistrationv1.MatchCondition {
	return admissionregistrationv1.MatchCondition{Name: name, Expression: expr}
}

// TestGetOrCompile_RaceWithAddExpression reproduces the crash described in #17463.
//
// It fires concurrent AddExpression writers (simulating informer resync callbacks)
// against a concurrent GetOrCompile reader (simulating the webhook-controller
// reconcile path on the elected leader).  Without the fix the race detector or Go's
// runtime map-protection fires within milliseconds; with the fix the test completes
// cleanly.
func TestGetOrCompile_RaceWithAddExpression(t *testing.T) {
	cache := NewExpressionCache()

	// Pre-seed a handful of expressions so preexistingExpressions is non-empty,
	// which is what makes the map-read in validateMatchConditionsExpression
	// (matchconditions.go:68) interesting.
	for i := 0; i < 5; i++ {
		cache.AddExpression(makeCondition(
			"seed",
			"true", // simple expression that always compiles
		))
	}

	conditions := []admissionregistrationv1.MatchCondition{
		makeCondition("c1", "object.metadata.name.startsWith('foo') && object.metadata.namespace in ['a','b','c','d','e']"),
		makeCondition("c2", "has(object.metadata.labels) && object.metadata.labels.exists(k, k.startsWith('app'))"),
		makeCondition("c3", "request.userInfo.username != 'system:anonymous' && request.userInfo.groups.exists(g, g == 'system:masters')"),
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer goroutines — simulate informer Add/Update/Delete callbacks.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					for _, c := range conditions {
						cache.AddExpression(c)
					}
				}
			}
		}()
	}

	// Reader goroutine — simulates the leader's reconcile path.
	wg.Add(1)
	go func() {
		defer wg.Done()
		deadline := time.After(500 * time.Millisecond)
		for {
			select {
			case <-deadline:
				close(stop)
				return
			default:
				cache.ValidateMatchConditions(conditions)
			}
		}
	}()

	wg.Wait()
}

// TestGetOrCompile_ConcurrentCompileSameKey checks that two goroutines compiling
// the same key simultaneously produce a consistent result and don't double-write
// in a racy way — the check-then-store in GetOrCompile prefers an already-stored
// entry.
func TestGetOrCompile_ConcurrentCompileSameKey(t *testing.T) {
	cache := NewExpressionCache()
	cond := makeCondition("dup", "true")

	var wg sync.WaitGroup
	results := make([]*compiledExpression, 10)

	for i := 0; i < 10; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = cache.GetOrCompile(cond)
		}()
	}
	wg.Wait()

	for i, r := range results {
		if r == nil {
			t.Errorf("goroutine %d got nil result", i)
		}
		if !r.isValid {
			t.Errorf("goroutine %d got invalid result: %v", i, r.errors)
		}
	}
}