package store

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	loaders "github.com/kyverno/kyverno/pkg/engine/context/loaders"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"k8s.io/apimachinery/pkg/runtime"
)

type fakeLoaderStore struct {
	entries map[string]gctxstore.Entry
}

func (f *fakeLoaderStore) Get(key string) (gctxstore.Entry, bool) {
	e, ok := f.entries[key]
	return e, ok
}

type staticGctxEntry struct {
	val interface{}
	err error
}

func (s *staticGctxEntry) Get(string) (interface{}, error) {
	return s.val, s.err
}

func (s *staticGctxEntry) Stop() {}

func TestDelegatingGCtxStore_MockWinsRealFallback(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{"only": "mock"})
	mock := NewMockGCtxStore([]v1alpha1.GlobalContextEntryValue{{
		Name: "mocked",
		Data: runtime.RawExtension{Raw: raw},
	}})
	real := &fakeLoaderStore{entries: map[string]gctxstore.Entry{
		"other": &staticGctxEntry{val: "from-real"},
	}}
	d := newDelegatingGCtxStore(mock, real)

	e, ok := d.Get("mocked")
	if !ok {
		t.Fatal("expected mock hit")
	}
	v, err := e.Get("")
	if err != nil || v.(map[string]interface{})["only"] != "mock" {
		t.Fatalf("%v %v", v, err)
	}

	e, ok = d.Get("other")
	if !ok {
		t.Fatal("expected real fallback")
	}
	v, err = e.Get("")
	if err != nil || v != "from-real" {
		t.Fatalf("%v %v", v, err)
	}

	_, ok = d.Get("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestMockEntry_GetProjection(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"items": []interface{}{1, 2},
	})
	m := NewMockGCtxStore([]v1alpha1.GlobalContextEntryValue{{
		Name: "g",
		Data: runtime.RawExtension{Raw: raw},
		Projections: []v1alpha1.GlobalContextProjection{
			{Name: "items", Path: "items"},
		},
	}})
	ent, ok := m.Get("g")
	if !ok {
		t.Fatal()
	}
	v, err := ent.Get("items")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.([]interface{})) != 2 {
		t.Fatalf("%#v", v)
	}
	_, err = ent.Get("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockEntry_GetProjectionNotObject(t *testing.T) {
	e := &mockEntry{value: "scalar"}
	_, err := e.Get("x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMockErrorEntry(t *testing.T) {
	m := NewMockGCtxStore([]v1alpha1.GlobalContextEntryValue{{
		Name:        "bad",
		Data:        runtime.RawExtension{},
		Projections: []v1alpha1.GlobalContextProjection{{Name: "n", Path: "p"}},
	}})
	ent, ok := m.Get("bad")
	if !ok {
		t.Fatal()
	}
	_, err := ent.Get("")
	if err == nil {
		t.Fatal("expected resolution error")
	}
}

func TestMockGCtxStore_ResourcesBackedGet(t *testing.T) {
	dep1, _ := json.Marshal(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "dep-1", "namespace": "default"},
	})
	dep2, _ := json.Marshal(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "dep-2", "namespace": "default"},
	})
	m := NewMockGCtxStore([]v1alpha1.GlobalContextEntryValue{{
		Name: "inline-deps",
		Resources: []runtime.RawExtension{
			{Raw: dep1},
			{Raw: dep2},
		},
	}})
	ent, ok := m.Get("inline-deps")
	if !ok {
		t.Fatal("expected entry")
	}
	v, err := ent.Get("")
	if err != nil {
		t.Fatal(err)
	}
	list, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}
	for i, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("resources[%d]: expected map, got %T", i, item)
		}
		if obj["kind"] != "Deployment" {
			t.Fatalf("resources[%d]: expected Deployment, got %v", i, obj["kind"])
		}
	}
}

func TestMockGCtxStore_ResourcesBackedProjection(t *testing.T) {
	dep, _ := json.Marshal(map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "dep-1"},
	})
	m := NewMockGCtxStore([]v1alpha1.GlobalContextEntryValue{{
		Name: "proj-deps",
		Resources: []runtime.RawExtension{
			{Raw: dep},
		},
		Projections: []v1alpha1.GlobalContextProjection{
			{Name: "names", Path: "[*].metadata.name"},
		},
	}})
	ent, ok := m.Get("proj-deps")
	if !ok {
		t.Fatal("expected entry")
	}
	v, err := ent.Get("names")
	if err != nil {
		t.Fatal(err)
	}
	names, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v)
	}
	if len(names) != 1 || names[0] != "dep-1" {
		t.Fatalf("unexpected names: %v", names)
	}
}

var _ loaders.Store = (*mockGCtxStore)(nil)
var _ loaders.Store = (*delegatingGCtxStore)(nil)
