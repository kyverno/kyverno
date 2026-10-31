package v2

import (
	"encoding/json"
	"testing"
)

func TestGlobalContextEntryStatusDeepCopyPreservesLegacyReady(t *testing.T) {
	t.Parallel()

	var status GlobalContextEntryStatus
	if err := json.Unmarshal([]byte(`{"ready":true}`), &status); err != nil {
		t.Fatal(err)
	}
	copied := status.DeepCopy()
	if copied.Ready == nil {
		t.Fatal("deep copy lost the legacy readiness value")
	}
	*copied.Ready = false
	if status.Ready == nil || !*status.Ready {
		t.Fatal("changing copied readiness changed the original status")
	}
}

func TestGlobalContextEntryStatusUsesModernReadiness(t *testing.T) {
	t.Parallel()

	var status GlobalContextEntryStatus
	if err := json.Unmarshal([]byte(`{"ready":false,"conditions":[{"type":"Ready","status":"True"}]}`), &status); err != nil {
		t.Fatal(err)
	}
	if !status.IsReady() {
		t.Fatal("legacy ready=false must not override a true Ready condition")
	}
	status.SetReady(false, "refresh failed")
	if status.IsReady() || status.Ready != nil {
		t.Fatal("setting conditions must update readiness and clear the deprecated field")
	}
}
