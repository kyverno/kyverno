package inspect

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func sampleConfig() bundle.Config {
	return bundle.Config{
		FormatVersion:  "1.0.0",
		Name:           "baseline",
		Version:        "1.0.0",
		Description:    "Baseline policies",
		KyvernoVersion: ">=1.20",
		Source:         &bundle.Source{URL: "https://github.com/acme/policies", Revision: "3f2c1a9"},
		Created:        "2026-10-01T00:00:00Z",
		Sets: []bundle.ConfigSet{
			{Name: "policies", Type: "policies"},
			{Name: "exceptions", Type: "exceptions"},
		},
		Resources: []bundle.ResourceEntry{
			{APIVersion: "policies.kyverno.io/v1beta1", Kind: "ValidatingPolicy", Name: "require-labels", Path: "policies/require-labels.yaml", Set: "policies"},
			{APIVersion: "policies.kyverno.io/v1beta1", Kind: "PolicyException", Namespace: "team-a", Name: "check-pod", Path: "exceptions/team-a-exceptions.yaml", Set: "exceptions"},
		},
	}
}

func sampleReport(cfg bundle.Config) Report {
	configBytes, _ := json.Marshal(cfg)
	return Report{
		Reference:       "ghcr.io/acme/baseline:v1.0.0",
		Digest:          "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		PinnedReference: "ghcr.io/acme/baseline@sha256:1111111111111111111111111111111111111111111111111111111111111111",
		Manifest: ManifestSummary{
			MediaType: "application/vnd.oci.image.manifest.v1+json",
			Config:    BlobSummary{MediaType: internal.ConfigMediaType, Digest: "sha256:aaa", Size: 10},
			Layers: []BlobSummary{
				{MediaType: internal.ContentMediaType, Digest: "sha256:2222222222222222222222222222222222222222222222222222222222222222", Size: 2048},
			},
		},
		Annotations: map[string]string{
			internal.AnnotationFormatVersion: "1.0.0",
			internal.AnnotationName:          "baseline",
			internal.AnnotationOCITitle:      "baseline",
		},
		Config:        configBytes,
		Compatibility: buildCompatibility(cfg.KyvernoVersion, "1.20.0"),
	}
}

// TestWriteTextMatchesLayout pins D2's exact text layout byte-for-byte: column order, the
// empty-namespace "-" substitution, the sets list, and the resource table header, so a silent
// reordering of the primary human-facing output fails this test instead of going unnoticed.
func TestWriteTextMatchesLayout(t *testing.T) {
	cfg := sampleConfig()
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))

	want := "Reference:      ghcr.io/acme/baseline:v1.0.0\n" +
		"Digest:         sha256:1111111111111111111111111111111111111111111111111111111111111111\n" +
		"Pinned:         ghcr.io/acme/baseline@sha256:1111111111111111111111111111111111111111111111111111111111111111\n" +
		"Format version: 1.0.0\n" +
		"Name:           baseline\n" +
		"Version:        1.0.0\n" +
		"Description:    Baseline policies\n" +
		"Kyverno:        >=1.20 (satisfied by 1.20.0)\n" +
		"Source:         https://github.com/acme/policies @ 3f2c1a9\n" +
		"Created:        2026-10-01T00:00:00Z\n" +
		"Layer:          application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip sha256:2222222222222222222222222222222222222222222222222222222222222222 (2048 bytes)\n" +
		"Sets:\n" +
		"  policies (policies)\n" +
		"  exceptions (exceptions)\n" +
		"Resources (2):\n" +
		"  APIVERSION                   KIND              NAMESPACE  NAME            SET         PATH\n" +
		"  policies.kyverno.io/v1beta1  ValidatingPolicy  -          require-labels  policies    policies/require-labels.yaml\n" +
		"  policies.kyverno.io/v1beta1  PolicyException   team-a     check-pod       exceptions  exceptions/team-a-exceptions.yaml\n" +
		"Annotations:\n" +
		"  io.kyverno.bundle.format-version: 1.0.0\n" +
		"  io.kyverno.bundle.name: baseline\n" +
		"  org.opencontainers.image.title: baseline\n"
	assert.Equal(t, want, out.String())
}

// TestWriteTextOmitsKyvernoLineWhenUndeclared pins D2's rule that the Kyverno: line is omitted
// entirely when the config declares no constraint.
func TestWriteTextOmitsKyvernoLineWhenUndeclared(t *testing.T) {
	cfg := sampleConfig()
	cfg.KyvernoVersion = ""
	r := sampleReport(cfg)
	r.Compatibility = buildCompatibility("", "1.20.0")

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	assert.NotContains(t, out.String(), "Kyverno:")
}

// TestWriteTextShowsNoneForEmptyAnnotations pins the "Annotations:    none" rule.
func TestWriteTextShowsNoneForEmptyAnnotations(t *testing.T) {
	cfg := sampleConfig()
	r := sampleReport(cfg)
	r.Annotations = nil

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	assert.Contains(t, out.String(), "Annotations:    none")
}

// TestWriteTextPathShowsDocumentIndex pins the "path (document N)" rule for a non-zero
// documentIndex, leaving index-0 entries printing the bare path.
func TestWriteTextPathShowsDocumentIndex(t *testing.T) {
	cfg := sampleConfig()
	cfg.Resources = []bundle.ResourceEntry{
		{APIVersion: "policies.kyverno.io/v1beta1", Kind: "ValidatingPolicy", Name: "a", Path: "multi.yaml", DocumentIndex: 0, Set: "policies"},
		{APIVersion: "policies.kyverno.io/v1beta1", Kind: "ValidatingPolicy", Name: "b", Path: "multi.yaml", DocumentIndex: 1, Set: "policies"},
	}
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	text := out.String()
	assert.Contains(t, text, "multi.yaml (document 1)")
}

// TestWriteJSONRoundTripsConfigContent pins D2: Report.Config is json.RawMessage, so -o json
// reproduces the config blob's content faithfully (MarshalIndent re-indents and HTML-escapes it,
// so it's not byte-for-byte), and the whole report round-trips.
func TestWriteJSONRoundTripsConfigContent(t *testing.T) {
	cfg := sampleConfig()
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeJSON(&out, r))

	var got Report
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, r.Digest, got.Digest)
	assert.JSONEq(t, string(r.Config), string(got.Config))
}

// TestWriteYAMLParses pins -o yaml producing valid YAML carrying the same content as JSON.
func TestWriteYAMLParses(t *testing.T) {
	cfg := sampleConfig()
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeYAML(&out, r))

	var got map[string]interface{}
	require.NoError(t, yaml.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, r.Digest, got["digest"])
}

// TestBuildCompatibilityStates pins all five Result values, including "invalid" distinguished
// from "unknown": a malformed constraint (nothing to check, because the declaration is garbage)
// is not the same state as a constraint that can't be checked only because CheckedAgainst isn't a
// release version.
func TestBuildCompatibilityStates(t *testing.T) {
	assert.Equal(t, "undeclared", buildCompatibility("", "1.20.0").Result)
	assert.Equal(t, "satisfied", buildCompatibility(">=1.20", "1.20.0").Result)
	assert.Equal(t, "unsatisfied", buildCompatibility(">=99.0.0", "1.20.0").Result)
	assert.Equal(t, "unknown", buildCompatibility(">=1.20", "not-a-release").Result)
	invalid := buildCompatibility("not a valid constraint!!!", "1.20.0")
	assert.Equal(t, "invalid", invalid.Result)
	assert.Equal(t, "not a valid constraint!!!", invalid.KyvernoVersion)
}

// TestWriteTextSanitizesControlCharacters pins that config-supplied text carrying a control
// character (here, ESC, the character an ANSI escape sequence starts with) is neutralised rather
// than printed raw: a registry is attacker-controlled input, and an unescaped ESC could rewrite or
// hide terminal output.
func TestWriteTextSanitizesControlCharacters(t *testing.T) {
	cfg := sampleConfig()
	cfg.Name = "baseline\x1b[31mRED\x1b[0m"
	cfg.Description = "line one\x1bline two"
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	text := out.String()
	assert.NotContains(t, text, "\x1b")
	assert.Contains(t, text, `baseline\x1b[31mRED\x1b[0m`)
	assert.Contains(t, text, `line one\x1bline two`)
}

// TestWriteTextSanitizesBidiFormatting pins the Trojan Source fix (CVE-2021-42574): U+202E
// RIGHT-TO-LEFT OVERRIDE is not a C0/C1 control character, but a terminal applying bidi formatting
// would still reorder displayed text around it, so it must be escaped the same as ESC. Ordinary
// non-ASCII text (accented Latin, CJK, emoji) must survive untouched.
func TestWriteTextSanitizesBidiFormatting(t *testing.T) {
	cfg := sampleConfig()
	cfg.Name = "baseline‮EDIR‬"
	cfg.Description = "café 中文 \U0001F600"
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	text := out.String()
	assert.NotContains(t, text, "‮")
	assert.NotContains(t, text, "‬")
	assert.Contains(t, text, "baseline\\u202eEDIR\\u202c")
	assert.Contains(t, text, "café 中文 \U0001F600")
}

// TestAnnotationWarningsReportsMismatchOnly pins the exact message and that an absent annotation
// (as opposed to a disagreeing one) never produces a warning.
func TestAnnotationWarningsReportsMismatchOnly(t *testing.T) {
	cfg := bundle.Config{FormatVersion: "1.0.0", Name: "baseline"}
	annotations := map[string]string{
		internal.AnnotationFormatVersion: "1.0.0",
		internal.AnnotationName:          "other-name",
	}
	warnings := annotationWarnings(annotations, cfg)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], internal.AnnotationName)
	assert.Contains(t, warnings[0], `is "other-name"`)
	assert.Contains(t, warnings[0], `the config says "baseline"`)
}

func TestAnnotationWarningsEmptyWhenNoAnnotations(t *testing.T) {
	cfg := bundle.Config{FormatVersion: "1.0.0", Name: "baseline"}
	assert.Empty(t, annotationWarnings(nil, cfg))
}

func TestSourceLineVariants(t *testing.T) {
	assert.Equal(t, "", sourceLine(nil))
	assert.Equal(t, "", sourceLine(&bundle.Source{}))
	assert.Equal(t, "https://x", sourceLine(&bundle.Source{URL: "https://x"}))
	assert.Equal(t, "abc123", sourceLine(&bundle.Source{Revision: "abc123"}))
	assert.Equal(t, "https://x @ abc123", sourceLine(&bundle.Source{URL: "https://x", Revision: "abc123"}))
}
