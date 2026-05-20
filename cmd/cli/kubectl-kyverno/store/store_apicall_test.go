package store

import (
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
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
