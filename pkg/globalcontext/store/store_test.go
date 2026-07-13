package store

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockEntry implements Entry interface for testing
type mockEntry struct {
	data    map[string]any
	stopped bool
	mu      sync.Mutex
}

func newMockEntry(data map[string]any) *mockEntry {
	return &mockEntry{data: data}
}

func (m *mockEntry) Get(projection string) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[projection], nil
}

func (m *mockEntry) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

func (m *mockEntry) isStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func TestNew_CreatesEmptyStore(t *testing.T) {
	s := New(0)
	assert.NotNil(t, s, "New() should return a non-nil store")

	// Verify store is empty by checking a random key
	entry, exists := s.Get("nonexistent")
	assert.False(t, exists, "new store should not contain any entries")
	assert.Nil(t, entry, "entry should be nil for nonexistent key")
}

func TestStore_SetAndGet_BasicOperations(t *testing.T) {
	s := New(0)
	entry := newMockEntry(map[string]any{"foo": "bar"})

	// Set an entry
	assert.NoError(t, s.Set("key1", entry))

	// Retrieve it
	retrieved, exists := s.Get("key1")
	assert.True(t, exists, "entry should exist after Set")
	assert.Same(t, entry, retrieved, "retrieved entry should be the same instance")

	// Verify entry data
	val, err := retrieved.Get("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", val)
}

func TestStore_SetOverwrite_StopsOldEntry(t *testing.T) {
	s := New(0)
	oldEntry := newMockEntry(map[string]any{"version": "old"})
	newEntry := newMockEntry(map[string]any{"version": "new"})

	// Set initial entry
	assert.NoError(t, s.Set("config", oldEntry))

	// Overwrite with new entry
	assert.NoError(t, s.Set("config", newEntry))

	// Old entry should be stopped
	assert.True(t, oldEntry.isStopped(), "old entry should be stopped when overwritten")
	assert.False(t, newEntry.isStopped(), "new entry should not be stopped")

	// Store should return new entry
	retrieved, exists := s.Get("config")
	assert.True(t, exists)
	assert.Same(t, newEntry, retrieved, "store should return the new entry")
}

func TestStore_Get_ReturnsNilForMissingKey(t *testing.T) {
	s := New(0)

	entry, exists := s.Get("missing-key")
	assert.False(t, exists, "exists should be false for missing key")
	assert.Nil(t, entry, "entry should be nil for missing key")
}

func TestStore_Delete_RemovesEntryAndStopsIt(t *testing.T) {
	s := New(0)
	entry := newMockEntry(map[string]any{"data": "value"})

	assert.NoError(t, s.Set("to-delete", entry))

	// Verify it exists
	_, exists := s.Get("to-delete")
	assert.True(t, exists, "entry should exist before deletion")

	// Delete it
	s.Delete("to-delete")

	// Verify it's gone
	_, exists = s.Get("to-delete")
	assert.False(t, exists, "entry should not exist after deletion")

	// Verify entry was stopped
	assert.True(t, entry.isStopped(), "entry should be stopped after deletion")
}

func TestStore_Delete_HandlesNonexistentKey(t *testing.T) {
	s := New(0)

	// Should not panic when deleting nonexistent key
	assert.NotPanics(t, func() {
		s.Delete("nonexistent")
	}, "deleting nonexistent key should not panic")
}

func TestStore_Delete_HandlesNilEntry(t *testing.T) {
	s := New(0).(*store)

	// Manually set nil to simulate edge case
	s.Lock()
	s.store["nil-key"] = nil
	s.Unlock()

	// Should not panic
	assert.NotPanics(t, func() {
		s.Delete("nil-key")
	}, "deleting nil entry should not panic")
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := New(0)
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			entry := newMockEntry(map[string]any{"id": id})
			_ = s.Set("shared-key", entry)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Get("shared-key")
		}()
	}

	// Concurrent deletes and sets
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			s.Delete("shared-key")
		}(i)
		go func(id int) {
			defer wg.Done()
			entry := newMockEntry(map[string]any{"id": id})
			_ = s.Set("shared-key", entry)
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions or panics occur
}

func TestStore_MultipleKeys(t *testing.T) {
	s := New(0)

	entries := make(map[string]*mockEntry)
	keys := []string{"alpha", "beta", "gamma", "delta"}

	// Set multiple entries
	for _, key := range keys {
		entry := newMockEntry(map[string]any{"name": key})
		entries[key] = entry
		assert.NoError(t, s.Set(key, entry))
	}

	// Verify all entries exist and are correct
	for _, key := range keys {
		retrieved, exists := s.Get(key)
		assert.True(t, exists, "entry %s should exist", key)
		assert.Same(t, entries[key], retrieved, "entry %s should match", key)
	}

	// Delete one
	s.Delete("beta")

	// Verify beta is gone but others remain
	_, exists := s.Get("beta")
	assert.False(t, exists, "beta should be deleted")
	assert.True(t, entries["beta"].isStopped(), "beta should be stopped")

	for _, key := range []string{"alpha", "gamma", "delta"} {
		_, exists := s.Get(key)
		assert.True(t, exists, "%s should still exist", key)
		assert.False(t, entries[key].isStopped(), "%s should not be stopped", key)
	}
}

func TestStore_SetNilEntry(t *testing.T) {
	s := New(0)

	// Setting nil should work without panic
	assert.NotPanics(t, func() {
		_ = s.Set("nil-entry", nil)
	}, "setting nil entry should not panic")

	entry, exists := s.Get("nil-entry")
	assert.True(t, exists, "nil entry should exist in store")
	assert.Nil(t, entry, "retrieved entry should be nil")
}

func TestStore_OverwriteNilWithValue(t *testing.T) {
	s := New(0)

	// Set nil first
	assert.NoError(t, s.Set("key", nil))

	// Overwrite with real entry
	entry := newMockEntry(map[string]any{"data": "test"})
	assert.NotPanics(t, func() {
		_ = s.Set("key", entry)
	}, "overwriting nil entry should not panic")

	retrieved, exists := s.Get("key")
	assert.True(t, exists)
	assert.Same(t, entry, retrieved)
}

func TestNew_ZeroMaxEntries_Unbounded(t *testing.T) {
	s := New(0)

	for i := 0; i < 1000; i++ {
		entry := newMockEntry(map[string]any{"i": i})
		assert.NoError(t, s.Set(strconv.Itoa(i), entry))
	}
}

func TestStore_Set_RejectsWhenAtCapacity(t *testing.T) {
	s := New(3)

	assert.NoError(t, s.Set("a", newMockEntry(nil)))
	assert.NoError(t, s.Set("b", newMockEntry(nil)))
	assert.NoError(t, s.Set("c", newMockEntry(nil)))

	rejected := newMockEntry(nil)
	err := s.Set("d", rejected)
	assert.Error(t, err, "Set should reject when the store is at capacity")
	assert.True(t, errors.Is(err, ErrStoreFull), "error should be ErrStoreFull")

	_, exists := s.Get("d")
	assert.False(t, exists, "rejected entry should not be stored")
	assert.False(t, rejected.isStopped(), "rejected entry should not be stopped")

	for _, key := range []string{"a", "b", "c"} {
		_, exists := s.Get(key)
		assert.True(t, exists, "existing entry %q should be preserved after a rejected Set", key)
	}
}

func TestStore_Set_OverwriteAtCapacity_Allowed(t *testing.T) {
	s := New(3)

	oldEntry := newMockEntry(map[string]any{"v": "old"})
	assert.NoError(t, s.Set("a", oldEntry))
	assert.NoError(t, s.Set("b", newMockEntry(nil)))
	assert.NoError(t, s.Set("c", newMockEntry(nil)))

	newEntry := newMockEntry(map[string]any{"v": "new"})
	assert.NoError(t, s.Set("a", newEntry), "overwriting an existing key should be allowed at capacity")

	assert.True(t, oldEntry.isStopped(), "old entry should be stopped when overwritten")
	retrieved, exists := s.Get("a")
	assert.True(t, exists)
	assert.Same(t, newEntry, retrieved)
}
