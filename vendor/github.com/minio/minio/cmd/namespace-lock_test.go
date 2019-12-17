/*
 * MinIO Cloud Storage, (C) 2016 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"testing"
	"time"
)

// WARNING:
//
// Expected source line number is hard coded, 32, in the
// following test. Adding new code before this test or changing its
// position will cause the line number to change and the test to FAIL
// Tests getSource().
func TestGetSource(t *testing.T) {
	currentSource := func() string { return getSource() }
	gotSource := currentSource()
	// Hard coded line number, 32, in the "expectedSource" value
	expectedSource := "[namespace-lock_test.go:32:TestGetSource()]"
	if gotSource != expectedSource {
		t.Errorf("expected : %s, got : %s", expectedSource, gotSource)
	}
}

// Tests functionality provided by namespace lock.
func TestNamespaceLockTest(t *testing.T) {
	isDistXL := false
	nsMutex := newNSLock(isDistXL)

	// List of test cases.
	testCases := []struct {
		lk               func(s1, s2, s3 string, t time.Duration) bool
		unlk             func(s1, s2, s3 string)
		rlk              func(s1, s2, s3 string, t time.Duration) bool
		runlk            func(s1, s2, s3 string)
		lockedRefCount   uint
		unlockedRefCount uint
		shouldPass       bool
	}{
		{
			lk:               nsMutex.Lock,
			unlk:             nsMutex.Unlock,
			lockedRefCount:   1,
			unlockedRefCount: 0,
			shouldPass:       true,
		},
		{
			rlk:              nsMutex.RLock,
			runlk:            nsMutex.RUnlock,
			lockedRefCount:   4,
			unlockedRefCount: 2,
			shouldPass:       true,
		},
		{
			rlk:              nsMutex.RLock,
			runlk:            nsMutex.RUnlock,
			lockedRefCount:   1,
			unlockedRefCount: 0,
			shouldPass:       true,
		},
	}

	// Run all test cases.

	// Write lock tests.
	testCase := testCases[0]
	if !testCase.lk("a", "b", "c", 60*time.Second) { // lock once.
		t.Fatalf("Failed to acquire lock")
	}
	nsLk, ok := nsMutex.lockMap[nsParam{"a", "b"}]
	if !ok && testCase.shouldPass {
		t.Errorf("Lock in map missing.")
	}
	// Validate locked ref count.
	if testCase.lockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 1, testCase.lockedRefCount, nsLk.ref)
	}
	testCase.unlk("a", "b", "c") // unlock once.
	if testCase.unlockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 1, testCase.unlockedRefCount, nsLk.ref)
	}
	_, ok = nsMutex.lockMap[nsParam{"a", "b"}]
	if ok && !testCase.shouldPass {
		t.Errorf("Lock map found after unlock.")
	}

	// Read lock tests.
	testCase = testCases[1]
	if !testCase.rlk("a", "b", "c", 60*time.Second) { // lock once.
		t.Fatalf("Failed to acquire first read lock")
	}
	if !testCase.rlk("a", "b", "c", 60*time.Second) { // lock second time.
		t.Fatalf("Failed to acquire second read lock")
	}
	if !testCase.rlk("a", "b", "c", 60*time.Second) { // lock third time.
		t.Fatalf("Failed to acquire third read lock")
	}
	if !testCase.rlk("a", "b", "c", 60*time.Second) { // lock fourth time.
		t.Fatalf("Failed to acquire fourth read lock")
	}
	nsLk, ok = nsMutex.lockMap[nsParam{"a", "b"}]
	if !ok && testCase.shouldPass {
		t.Errorf("Lock in map missing.")
	}
	// Validate locked ref count.
	if testCase.lockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 1, testCase.lockedRefCount, nsLk.ref)
	}

	testCase.runlk("a", "b", "c") // unlock once.
	testCase.runlk("a", "b", "c") // unlock second time.
	if testCase.unlockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 2, testCase.unlockedRefCount, nsLk.ref)
	}
	_, ok = nsMutex.lockMap[nsParam{"a", "b"}]
	if !ok && testCase.shouldPass {
		t.Errorf("Lock map not found.")
	}

	// Read lock 0 ref count.
	testCase = testCases[2]
	if !testCase.rlk("a", "c", "d", 60*time.Second) { // lock once.
		t.Fatalf("Failed to acquire read lock")
	}

	nsLk, ok = nsMutex.lockMap[nsParam{"a", "c"}]
	if !ok && testCase.shouldPass {
		t.Errorf("Lock in map missing.")
	}
	// Validate locked ref count.
	if testCase.lockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 3, testCase.lockedRefCount, nsLk.ref)
	}
	testCase.runlk("a", "c", "d") // unlock once.
	if testCase.unlockedRefCount != nsLk.ref && testCase.shouldPass {
		t.Errorf("Test %d fails, expected to pass. Wanted ref count is %d, got %d", 3, testCase.unlockedRefCount, nsLk.ref)
	}
	_, ok = nsMutex.lockMap[nsParam{"a", "c"}]
	if ok && !testCase.shouldPass {
		t.Errorf("Lock map not found.")
	}
}

func TestNamespaceLockTimedOut(t *testing.T) {
	isDistXL := false
	nsMutex := newNSLock(isDistXL)
	// Get write lock
	if !nsMutex.Lock("my-bucket", "my-object", "abc", 60*time.Second) {
		t.Fatalf("Failed to acquire lock")
	}

	// Second attempt for write lock on same resource should time out
	locked := nsMutex.Lock("my-bucket", "my-object", "def", 1*time.Second)
	if locked {
		t.Fatalf("Should not have acquired lock")
	}

	// Read lock on same resource should also time out
	locked = nsMutex.RLock("my-bucket", "my-object", "def", 1*time.Second)
	if locked {
		t.Fatalf("Should not have acquired read lock while write lock is active")
	}

	// Release write lock
	nsMutex.Unlock("my-bucket", "my-object", "abc")

	// Get read lock
	if !nsMutex.RLock("my-bucket", "my-object", "ghi", 60*time.Second) {
		t.Fatalf("Failed to acquire read lock")
	}

	// Write lock on same resource should time out
	locked = nsMutex.Lock("my-bucket", "my-object", "klm", 1*time.Second)
	if locked {
		t.Fatalf("Should not have acquired lock")
	}

	// 2nd read lock should be just fine
	if !nsMutex.RLock("my-bucket", "my-object", "nop", 60*time.Second) {
		t.Fatalf("Failed to acquire second read lock")
	}

	// Release both read locks
	nsMutex.RUnlock("my-bucket", "my-object", "ghi")
	nsMutex.RUnlock("my-bucket", "my-object", "nop")
}
