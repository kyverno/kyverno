package store

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
)

type mockGCtxStore struct {
	entries map[string]gctxstore.Entry
}

func NewMockGCtxStore(mocks []v1alpha1.MockGlobalContextEntry) *mockGCtxStore {
	entries := make(map[string]gctxstore.Entry, len(mocks))
	for _, m := range mocks {
		entries[m.Name] = &mockEntry{data: m.Data}
	}
	return &mockGCtxStore{entries: entries}
}

func (s *mockGCtxStore) Get(key string) (gctxstore.Entry, bool) {
	entry, ok := s.entries[key]
	return entry, ok
}

type mockEntry struct {
	data interface{}
}

func (e *mockEntry) Get(_ string) (interface{}, error) {
	return e.data, nil
}

func (e *mockEntry) Stop() {}
