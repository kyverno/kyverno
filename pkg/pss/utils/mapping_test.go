package utils

import "testing"

func TestPSSControlIDToName_ProcMountRestricted(t *testing.T) {
	if got := PSSControlIDToName("procMount_restricted"); got != "/proc Mount Type" {
		t.Fatalf("expected /proc Mount Type, got %q", got)
	}
	if len(PSS_controls["procMount_restricted"]) == 0 {
		t.Fatal("expected restricted fields for procMount_restricted")
	}
}
