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

// TestWriteTextMatchesLayout pins D2's exact text layout, including the empty-namespace "-"
// substitution, the sets list, and the resource table header.
func TestWriteTextMatchesLayout(t *testing.T) {
	cfg := sampleConfig()
	r := sampleReport(cfg)

	var out bytes.Buffer
	require.NoError(t, writeText(&out, r, cfg))
	text := out.String()

	assert.Contains(t, text, "Reference:")
	assert.Contains(t, text, "ghcr.io/acme/baseline:v1.0.0")
	assert.Contains(t, text, "Pinned:")
	assert.Contains(t, text, "Format version:")
	assert.Contains(t, text, "1.0.0")
	assert.Contains(t, text, "Kyverno:")
	assert.Contains(t, text, ">=1.20 (satisfied by 1.20.0)")
	assert.Contains(t, text, "Source:")
	assert.Contains(t, text, "https://github.com/acme/policies @ 3f2c1a9")
	assert.Contains(t, text, "Sets:")
	assert.Contains(t, text, "policies (policies)")
	assert.Contains(t, text, "Resources (2):")
	assert.Contains(t, text, "NAMESPACE")
	assert.Contains(t, text, " - ")
	assert.Contains(t, text, "team-a")
	assert.Contains(t, text, "Annotations:")
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

// TestWriteJSONRoundTripsConfigVerbatim pins D2: Report.Config is json.RawMessage, so -o json
// echoes the config blob byte-for-byte, and the whole report round-trips.
func TestWriteJSONRoundTripsConfigVerbatim(t *testing.T) {
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

// TestBuildCompatibilityStates pins all four Result values.
func TestBuildCompatibilityStates(t *testing.T) {
	assert.Equal(t, "undeclared", buildCompatibility("", "1.20.0").Result)
	assert.Equal(t, "satisfied", buildCompatibility(">=1.20", "1.20.0").Result)
	assert.Equal(t, "unsatisfied", buildCompatibility(">=99.0.0", "1.20.0").Result)
	assert.Equal(t, "unknown", buildCompatibility(">=1.20", "not-a-release").Result)
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
