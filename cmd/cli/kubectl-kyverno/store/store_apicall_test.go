package store

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStore_APICallAndGlobalContextRoundTrip(t *testing.T) {
	var s Store
	api := []v1alpha1.APICallResponseEntry{{URL: "https://x"}}
	gctx := []v1alpha1.GlobalContextEntryValue{{Name: "g"}}
	s.SetAPICallResponses(api)
	s.SetGlobalContextEntries(gctx)
	if len(s.GetAPICallResponses()) != 1 || s.GetAPICallResponses()[0].URL != "https://x" {
		t.Fatalf("%#v", s.GetAPICallResponses())
	}
	if len(s.GetGlobalContextEntries()) != 1 || s.GetGlobalContextEntries()[0].Name != "g" {
		t.Fatalf("%#v", s.GetGlobalContextEntries())
	}
}

func TestStore_GlobalContextResourcesRoundTrip(t *testing.T) {
	var s Store
	gctx := []v1alpha1.GlobalContextEntryValue{{
		Name: "inline",
		Resources: []runtime.RawExtension{
			{Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment"}`)},
			{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`)},
		},
	}}
	s.SetGlobalContextEntries(gctx)
	got := s.GetGlobalContextEntries()
	if len(got) != 1 || got[0].Name != "inline" {
		t.Fatalf("name: %#v", got)
	}
	if len(got[0].Resources) != 2 {
		t.Fatalf("resources count: %d", len(got[0].Resources))
	}
}

func TestStore_GlobalContextResourceFilesRoundTrip(t *testing.T) {
	var s Store
	gctx := []v1alpha1.GlobalContextEntryValue{{
		Name:          "from-files",
		ResourceFiles: []string{"deps.yaml", "configmaps.yaml"},
	}}
	s.SetGlobalContextEntries(gctx)
	got := s.GetGlobalContextEntries()
	if len(got) != 1 || got[0].Name != "from-files" {
		t.Fatalf("name: %#v", got)
	}
	if len(got[0].ResourceFiles) != 2 || got[0].ResourceFiles[0] != "deps.yaml" {
		t.Fatalf("resourceFiles: %v", got[0].ResourceFiles)
	}
}
