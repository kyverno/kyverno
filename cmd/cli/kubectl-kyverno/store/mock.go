package store

import (
	"fmt"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	loaders "github.com/kyverno/kyverno/pkg/engine/context/loaders"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
)

// delegatingGCtxStore tries test mocks first, then falls back to the real global context store.
type delegatingGCtxStore struct {
	mock *mockGCtxStore
	real loaders.Store
}

func newDelegatingGCtxStore(mock *mockGCtxStore, real loaders.Store) *delegatingGCtxStore {
	return &delegatingGCtxStore{mock: mock, real: real}
}

func (d *delegatingGCtxStore) Get(key string) (gctxstore.Entry, bool) {
	if d.mock != nil {
		if e, ok := d.mock.Get(key); ok {
			return e, true
		}
	}
	if d.real != nil {
		return d.real.Get(key)
	}
	return nil, false
}

type mockGCtxStore struct {
	entries map[string]gctxstore.Entry
}

func NewMockGCtxStore(mocks []v1alpha1.GlobalContextEntryValue) *mockGCtxStore {
	entries := make(map[string]gctxstore.Entry, len(mocks))
	for _, m := range mocks {
		payload, err := ResolveGlobalContextMockData(m)
		if err != nil {
			entries[m.Name] = &mockErrorEntry{err: err}
			continue
		}
		entries[m.Name] = &mockEntry{value: payload}
	}
	return &mockGCtxStore{entries: entries}
}

func (s *mockGCtxStore) Get(key string) (gctxstore.Entry, bool) {
	entry, ok := s.entries[key]
	return entry, ok
}

type mockEntry struct {
	value interface{}
}

func (e *mockEntry) Get(projection string) (interface{}, error) {
	if projection == "" {
		return e.value, nil
	}
	m, ok := e.value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("global context mock: projection %q requested but value is not an object", projection)
	}
	v, ok := m[projection]
	if !ok {
		return nil, fmt.Errorf("global context mock: projection %q not found", projection)
	}
	return v, nil
}

func (e *mockEntry) Stop() {}

type mockErrorEntry struct {
	err error
}

func (e *mockErrorEntry) Get(_ string) (interface{}, error) {
	return nil, e.err
}

func (e *mockErrorEntry) Stop() {}
