package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes contents (joined with "\n" plus a trailing newline) to
// dir/name and returns the full path.
func writeFile(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// crdEntry builds a minimal synthetic "spec.versions[]" entry with the
// given version name and an extra marker line so tests can tell entries
// apart without parsing real CRD schemas.
func crdEntry(version, marker string) []string {
	return []string{
		"  - additionalPrinterColumns: []",
		"    marker: " + marker,
		"    name: " + version,
		"    schema:",
		"      openAPIV3Schema:",
		"        type: object",
		"    served: true",
		"    storage: true",
	}
}

func crdHeader() []string {
	return []string{
		"apiVersion: apiextensions.k8s.io/v1",
		"kind: CustomResourceDefinition",
		"metadata:",
		"  name: things.kyverno.io",
		"spec:",
		"  group: kyverno.io",
		"  names:",
		"    kind: Thing",
		"  scope: Namespaced",
		"  versions:",
	}
}

func crdText(marker ...struct{ version, marker string }) string {
	lines := crdHeader()
	for _, m := range marker {
		lines = append(lines, crdEntry(m.version, m.marker)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestDoSplice_VerbatimCopyWhenNoFreezeEntry(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(generated, "kyverno.io_things.yaml"),
		[]byte(crdText(struct{ version, marker string }{"v1", "fromgen"})), 0o644); err != nil {
		t.Fatalf("writing generated fixture: %v", err)
	}
	writeFile(t, target, "kyverno.io_things.yaml", "stale content that must be overwritten")

	if err := doSplice(generated, target, nil, nil); err != nil {
		t.Fatalf("doSplice: %v", err)
	}

	got := readFile(t, filepath.Join(target, "kyverno.io_things.yaml"))
	want := crdText(struct{ version, marker string }{"v1", "fromgen"})
	if got != want {
		t.Errorf("target file = %q, want %q", got, want)
	}
}

func TestDoSplice_FrozenEntryPreservedAndSorted(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()

	// generated has v2 only (v1beta1's Go type no longer emits anything).
	genLines := append(crdHeader(), crdEntry("v2", "fromgen-v2")...)
	writeFile(t, generated, "kyverno.io_updaterequests.yaml", genLines...)

	// target (checked-in) has v1beta1 then v2, with a trailing blank line
	// inside the v1beta1 entry that must survive the splice untouched.
	v1beta1 := crdEntry("v1beta1", "frozen-v1beta1")
	v1beta1 = append(v1beta1[:len(v1beta1)-1], "", v1beta1[len(v1beta1)-1])
	tgtLines := append(crdHeader(), v1beta1...)
	tgtLines = append(tgtLines, crdEntry("v2", "stale-v2")...)
	writeFile(t, target, "kyverno.io_updaterequests.yaml", tgtLines...)

	freeze := map[string]map[string]bool{"kyverno.io_updaterequests.yaml": {"v1beta1": true}}
	if err := doSplice(generated, target, nil, freeze); err != nil {
		t.Fatalf("doSplice: %v", err)
	}

	got := readFile(t, filepath.Join(target, "kyverno.io_updaterequests.yaml"))
	wantLines := append(crdHeader(), v1beta1...)
	wantLines = append(wantLines, crdEntry("v2", "fromgen-v2")...)
	want := strings.Join(wantLines, "\n") + "\n"
	if got != want {
		t.Errorf("spliced file =\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(got, "frozen-v1beta1") {
		t.Error("frozen v1beta1 marker lost")
	}
	if strings.Contains(got, "stale-v2") {
		t.Error("stale v2 entry was not replaced by generated output")
	}
}

func TestDoSplice_UnlistedNewVersionTracksGo(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()

	// generated introduces v3, which no --freeze entry names.
	genLines := append(crdHeader(), crdEntry("v2", "gen-v2")...)
	genLines = append(genLines, crdEntry("v3", "gen-v3")...)
	writeFile(t, generated, "kyverno.io_globalcontextentries.yaml", genLines...)

	tgtLines := append(crdHeader(), crdEntry("v2", "old-v2")...)
	writeFile(t, target, "kyverno.io_globalcontextentries.yaml", tgtLines...)

	if err := doSplice(generated, target, nil, nil); err != nil {
		t.Fatalf("doSplice: %v", err)
	}

	got := readFile(t, filepath.Join(target, "kyverno.io_globalcontextentries.yaml"))
	if !strings.Contains(got, "name: v3") {
		t.Error("new unlisted version v3 did not appear in the result (default-safe violated)")
	}
	if !strings.Contains(got, "gen-v2") || strings.Contains(got, "old-v2") {
		t.Error("v2 did not track the generated output")
	}
}

func TestDoSplice_FrozenVersionMissingFromTarget(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	genLines := append(crdHeader(), crdEntry("v2", "gen-v2")...)
	writeFile(t, generated, "kyverno.io_updaterequests.yaml", genLines...)
	// target has no v1beta1 entry at all.
	tgtLines := append(crdHeader(), crdEntry("v2", "tgt-v2")...)
	writeFile(t, target, "kyverno.io_updaterequests.yaml", tgtLines...)

	freeze := map[string]map[string]bool{"kyverno.io_updaterequests.yaml": {"v1beta1": true}}
	err := doSplice(generated, target, nil, freeze)
	if err == nil {
		t.Fatal("expected an error when the frozen version is missing from target, got nil")
	}
	if !strings.Contains(err.Error(), "v1beta1") {
		t.Errorf("error %q does not mention the missing frozen version", err)
	}
}

func TestDoSplice_NonFrozenTargetFileMissingFromGenerated(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	// generated is empty: the retained kind's Go type was removed.
	writeFile(t, target, "kyverno.io_updaterequests.yaml", crdText(struct{ version, marker string }{"v2", "x"}))

	err := doSplice(generated, target, nil, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	want := "retained CRD kyverno.io_updaterequests.yaml has no generated output; its Go type was removed"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

func TestDoSplice_FrozenFileNeverWritten(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	// Generated contains a (partial) counterpart; it must be ignored.
	if err := os.WriteFile(filepath.Join(generated, "kyverno.io_clusterpolicies.yaml"),
		[]byte(crdText(struct{ version, marker string }{"v2beta1", "from-gen-must-not-land"})), 0o644); err != nil {
		t.Fatalf("writing generated fixture: %v", err)
	}
	original := crdText(struct{ version, marker string }{"v2beta1", "frozen-original"})
	if err := os.WriteFile(filepath.Join(target, "kyverno.io_clusterpolicies.yaml"), []byte(original), 0o644); err != nil {
		t.Fatalf("writing target fixture: %v", err)
	}

	if err := doSplice(generated, target, map[string]bool{"kyverno.io_clusterpolicies.yaml": true}, nil); err != nil {
		t.Fatalf("doSplice: %v", err)
	}

	got := readFile(t, filepath.Join(target, "kyverno.io_clusterpolicies.yaml"))
	if got != original {
		t.Errorf("frozen file was modified: got %q, want unchanged %q", got, original)
	}
}

func TestDoSplice_FreezeKeyNotInGeneratedIsRejected(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	genLines := append(crdHeader(), crdEntry("v2", "gen-v2")...)
	writeFile(t, generated, "kyverno.io_updaterequests.yaml", genLines...)
	tgtLines := append(crdHeader(), crdEntry("v1beta1", "frozen")...)
	tgtLines = append(tgtLines, crdEntry("v2", "tgt-v2")...)
	writeFile(t, target, "kyverno.io_updaterequests.yaml", tgtLines...)

	// typo: "updaterequest" (missing the trailing "s") never matches a
	// generated file, so the real v1beta1 freeze must not be silently lost.
	freeze := map[string]map[string]bool{"kyverno.io_updaterequest.yaml": {"v1beta1": true}}
	err := doSplice(generated, target, nil, freeze)
	if err == nil {
		t.Fatal("expected an error for an unconsumed --freeze key, got nil")
	}
	if !strings.Contains(err.Error(), "kyverno.io_updaterequest.yaml") {
		t.Errorf("error %q does not name the unconsumed --freeze key", err)
	}
}

func TestDoSplice_FrozenFileMissingFromTarget(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()
	err := doSplice(generated, target, map[string]bool{"kyverno.io_clusterpolicies.yaml": true}, nil)
	if err == nil {
		t.Fatal("expected an error when a frozen file is absent from target, got nil")
	}
}

func TestEntryVersionName_ZeroOrTwoNameLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		entry []string
	}{
		{"zero name lines", []string{"  - foo: bar", "    baz: qux"}},
		{"two name lines", []string{"  - foo: bar", "    name: v1", "    name: v2"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := entryVersionName(tc.entry); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestDoExtract_ReturnsExactlyTheEntryBytes(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	tgtLines := append(crdHeader(), crdEntry("v1beta1", "frozen")...)
	tgtLines = append(tgtLines, crdEntry("v2", "live")...)
	writeFile(t, target, "kyverno.io_updaterequests.yaml", tgtLines...)

	got, err := doExtract(target, "kyverno.io_updaterequests.yaml", "v1beta1")
	if err != nil {
		t.Fatalf("doExtract: %v", err)
	}
	want := strings.Join(crdEntry("v1beta1", "frozen"), "\n")
	if got != want {
		t.Errorf("extract = %q, want %q", got, want)
	}
}

func TestDoExtract_VersionAbsent(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeFile(t, target, "kyverno.io_updaterequests.yaml", crdText(struct{ version, marker string }{"v2", "live"}))

	if _, err := doExtract(target, "kyverno.io_updaterequests.yaml", "v1beta1"); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// TestDoSplice_URShapedFixture mirrors the real UpdateRequest file: target
// has v1beta1 and v2, generated has only v2 with a changed field. The
// splice must confine the diff to the v2 entry and leave v1beta1 untouched.
func TestDoSplice_URShapedFixture(t *testing.T) {
	t.Parallel()
	generated, target := t.TempDir(), t.TempDir()

	v1beta1 := crdEntry("v1beta1", "frozen")
	oldV2 := crdEntry("v2", "old-field")
	newV2 := crdEntry("v2", "new-field")

	tgtLines := append(crdHeader(), v1beta1...)
	tgtLines = append(tgtLines, oldV2...)
	writeFile(t, target, "kyverno.io_updaterequests.yaml", tgtLines...)

	genLines := append(crdHeader(), newV2...)
	writeFile(t, generated, "kyverno.io_updaterequests.yaml", genLines...)

	freeze := map[string]map[string]bool{"kyverno.io_updaterequests.yaml": {"v1beta1": true}}
	if err := doSplice(generated, target, nil, freeze); err != nil {
		t.Fatalf("doSplice: %v", err)
	}

	got := readFile(t, filepath.Join(target, "kyverno.io_updaterequests.yaml"))
	if !strings.Contains(got, "frozen") {
		t.Error("v1beta1 bytes were not preserved")
	}
	if strings.Contains(got, "old-field") {
		t.Error("v2 entry did not pick up the generated change")
	}
	if !strings.Contains(got, "new-field") {
		t.Error("v2 entry missing the generated change")
	}
}
