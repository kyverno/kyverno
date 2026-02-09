package store

import (
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
	s := New()
	assert.NotNil(t, s, "New() should return a non-nil store")

	// Verify store is empty by checking a random key
	entry, exists := s.Get("nonexistent")
	assert.False(t, exists, "new store should not contain any entries")
	assert.Nil(t, entry, "entry should be nil for nonexistent key")
}

func TestStore_SetAndGet_BasicOperations(t *testing.T) {
	s := New()
	entry := newMockEntry(map[string]any{"foo": "bar"})

	// Set an entry
	s.Set("key1", entry)

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
	s := New()
	oldEntry := newMockEntry(map[string]any{"version": "old"})
	newEntry := newMockEntry(map[string]any{"version": "new"})

	// Set initial entry
	s.Set("config", oldEntry)

	// Overwrite with new entry
	s.Set("config", newEntry)

	// Old entry should be stopped
	assert.True(t, oldEntry.isStopped(), "old entry should be stopped when overwritten")
	assert.False(t, newEntry.isStopped(), "new entry should not be stopped")

	// Store should return new entry
	retrieved, exists := s.Get("config")
	assert.True(t, exists)
	assert.Same(t, newEntry, retrieved, "store should return the new entry")
}

func TestStore_Get_ReturnsNilForMissingKey(t *testing.T) {
	s := New()

	entry, exists := s.Get("missing-key")
	assert.False(t, exists, "exists should be false for missing key")
	assert.Nil(t, entry, "entry should be nil for missing key")
}

func TestStore_Delete_RemovesEntryAndStopsIt(t *testing.T) {
	s := New()
	entry := newMockEntry(map[string]any{"data": "value"})

	s.Set("to-delete", entry)

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
	s := New()

	// Should not panic when deleting nonexistent key
	assert.NotPanics(t, func() {
		s.Delete("nonexistent")
	}, "deleting nonexistent key should not panic")
}

func TestStore_Delete_HandlesNilEntry(t *testing.T) {
	s := New().(*store)

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
	s := New()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			entry := newMockEntry(map[string]any{"id": id})
			s.Set("shared-key", entry)
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
			s.Set("shared-key", entry)
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions or panics occur
}

func TestStore_MultipleKeys(t *testing.T) {
	s := New()

	entries := make(map[string]*mockEntry)
	keys := []string{"alpha", "beta", "gamma", "delta"}

	// Set multiple entries
	for _, key := range keys {
		entry := newMockEntry(map[string]any{"name": key})
		entries[key] = entry
		s.Set(key, entry)
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
	s := New()

	// Setting nil should work without panic
	assert.NotPanics(t, func() {
		s.Set("nil-entry", nil)
	}, "setting nil entry should not panic")

	entry, exists := s.Get("nil-entry")
	assert.True(t, exists, "nil entry should exist in store")
	assert.Nil(t, entry, "retrieved entry should be nil")
}

func TestStore_OverwriteNilWithValue(t *testing.T) {
	s := New()

	// Set nil first
	s.Set("key", nil)

	// Overwrite with real entry
	entry := newMockEntry(map[string]any{"data": "test"})
	assert.NotPanics(t, func() {
		s.Set("key", entry)
	}, "overwriting nil entry should not panic")

	retrieved, exists := s.Get("key")
	assert.True(t, exists)
	assert.Same(t, entry, retrieved)
}
