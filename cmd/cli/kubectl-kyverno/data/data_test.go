package data

import (
	"testing"

	"k8s.io/client-go/restmapper"
)

func TestCrds(t *testing.T) {
	fsys, err := Crds()
	if err != nil {
		t.Fatalf("Crds() returned error: %v", err)
	}
	if fsys == nil {
		t.Fatal("Crds() returned nil fs.FS")
	}
}

func TestAPIGroupResources(t *testing.T) {
	resources, err := APIGroupResources()
	if err != nil {
		t.Fatalf("APIGroupResources() returned error: %v", err)
	}
	if resources == nil {
		t.Fatal("APIGroupResources() returned nil")
	}
}

func TestNewCRDProcessor(t *testing.T) {
	resources := []*restmapper.APIGroupResources{
		{},
	}
	p := NewCRDProcessor(resources)
	if p == nil {
		t.Fatal("NewCRDProcessor() returned nil")
	}
	got := p.GetResourceGroups()
	if len(got) != len(resources) {
		t.Errorf("GetResourceGroups() len = %d, want %d", len(got), len(resources))
	}
}

func TestCRDProcessorUpdateAndGet(t *testing.T) {
	p := NewCRDProcessor(nil)
	if p.GetResourceGroups() != nil {
		t.Error("expected nil resource groups initially")
	}

	updated := []*restmapper.APIGroupResources{{}, {}}
	p.UpdateResourceGroups(updated)

	got := p.GetResourceGroups()
	if len(got) != len(updated) {
		t.Errorf("GetResourceGroups() len = %d, want %d", len(got), len(updated))
	}
}
