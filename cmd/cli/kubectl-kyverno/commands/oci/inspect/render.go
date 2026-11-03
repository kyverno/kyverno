package inspect

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal/bundle"
	"sigs.k8s.io/yaml"
)

// sanitize neutralises C0/C1 control characters (including ESC, so an ANSI escape sequence can't
// rewrite or hide terminal output) and Unicode format characters (unicode.Cf — bidi overrides and
// isolates, ALM, soft hyphen, BOM, and so on — so none of them can reorder or hide displayed text,
// the "Trojan Source" class, CVE-2021-42574) in a string the bundle's config or manifest
// annotations supply, since both come from the registry and are attacker-controlled input for
// anyone inspecting an untrusted bundle. unicode.Cf is the canonical predicate for this, rather
// than a hand-enumerated list of code points that has already needed two corrections: it covers
// every bidi control plus ones not previously named, such as U+061C ARABIC LETTER MARK, U+00AD
// SOFT HYPHEN, and U+FEFF BYTE ORDER MARK. The one deliberate exception is U+200D ZERO WIDTH
// JOINER: it is in Cf, but legitimate emoji sequences (a family or profession emoji) use it to
// join glyphs rather than reorder them, so it doesn't carry the same threat and escaping it would
// mangle ordinary text. Every other rune, including ordinary non-ASCII Unicode (accented Latin,
// CJK, emoji), passes through unchanged.
func sanitize(s string) string {
	const zeroWidthJoiner = 0x200d
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsControl(r):
			fmt.Fprintf(&b, "\\x%02x", r)
		case r != zeroWidthJoiner && unicode.Is(unicode.Cf, r):
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Report is inspect's -o json|yaml payload; text mode renders the same data (plus Config, read
// from md.Config rather than Report.Config) through writeText.
type Report struct {
	Reference       string            `json:"reference"`
	Digest          string            `json:"digest"`
	PinnedReference string            `json:"pinnedReference"`
	Manifest        ManifestSummary   `json:"manifest"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	Config          json.RawMessage   `json:"config"`
	Compatibility   Compatibility     `json:"compatibility"`
	Warnings        []string          `json:"warnings,omitempty"`
}

type ManifestSummary struct {
	MediaType string        `json:"mediaType"`
	Config    BlobSummary   `json:"config"`
	Layers    []BlobSummary `json:"layers"`
}

type BlobSummary struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// Compatibility is inspect's own compatibility summary. Result is one of "satisfied",
// "unsatisfied", "unknown" (CheckedAgainst isn't a release version), "invalid" (the config's own
// kyvernoVersion constraint doesn't parse as a semver constraint, named in KyvernoVersion), or
// "undeclared" (the config has no kyvernoVersion constraint); inspect is display-only and never
// fails on any of them.
type Compatibility struct {
	KyvernoVersion string `json:"kyvernoVersion,omitempty"`
	CheckedAgainst string `json:"checkedAgainst"`
	Result         string `json:"result"`
}

// buildReport assembles Report and the annotation/config mismatch warnings (shared between text
// mode's stderr lines and structured mode's Report.Warnings) from a fetched descriptor and the
// bundle metadata it describes. target is the Kyverno version to compare the config's declared
// constraint against (--kyverno-version, or the running CLI's own version).
func buildReport(ref name.Reference, desc *remote.Descriptor, md *bundle.Metadata, target string) (Report, []string) {
	manifest := md.Manifest
	pinned := ref.Context().Digest(desc.Digest.String()).Name()

	layers := make([]BlobSummary, 0, len(manifest.Layers))
	for _, l := range manifest.Layers {
		layers = append(layers, BlobSummary{
			MediaType: string(l.MediaType),
			Digest:    l.Digest.String(),
			Size:      l.Size,
		})
	}

	report := Report{
		Reference:       ref.Name(),
		Digest:          desc.Digest.String(),
		PinnedReference: pinned,
		Manifest: ManifestSummary{
			MediaType: string(manifest.MediaType),
			Config: BlobSummary{
				MediaType: string(manifest.Config.MediaType),
				Digest:    manifest.Config.Digest.String(),
				Size:      manifest.Config.Size,
			},
			Layers: layers,
		},
		Annotations: manifest.Annotations,
		Config:      json.RawMessage(md.RawConfig),
	}

	report.Compatibility = buildCompatibility(md.Config.KyvernoVersion, target)

	warnings := annotationWarnings(manifest.Annotations, md.Config)
	report.Warnings = warnings

	return report, warnings
}

func buildCompatibility(constraint, target string) Compatibility {
	c := Compatibility{CheckedAgainst: target}
	if constraint == "" {
		c.Result = "undeclared"
		return c
	}
	c.KyvernoVersion = constraint

	result, err := bundle.CheckKyvernoVersion(constraint, target)
	if err != nil {
		// The constraint itself doesn't parse, which is a different state from "nothing to check
		// against" (CheckedAgainst not a release version, the Unknown case below): the bundle's
		// own declaration is garbage, not merely unverifiable against this target.
		c.Result = "invalid"
		return c
	}
	switch result {
	case bundle.CompatibilitySatisfied:
		c.Result = "satisfied"
	case bundle.CompatibilityUnsatisfied:
		c.Result = "unsatisfied"
	default:
		c.Result = "unknown"
	}
	return c
}

// annotationWarnings compares the manifest's io.kyverno.bundle.* annotations against the config
// (bundle-spec.md section 4: annotations MUST agree with the config and the config is
// authoritative) and returns one message per mismatch, text identical to what both render modes
// surface. An absent annotation is not a mismatch: it's never required.
func annotationWarnings(annotations map[string]string, cfg bundle.Config) []string {
	var warnings []string
	check := func(key, configVal string) {
		annotationVal, ok := annotations[key]
		if !ok || annotationVal == configVal {
			return
		}
		warnings = append(warnings, fmt.Sprintf("annotation %s is %q but the config says %q; the config is authoritative", key, annotationVal, configVal))
	}
	check(internal.AnnotationFormatVersion, cfg.FormatVersion)
	check(internal.AnnotationName, cfg.Name)
	check(internal.AnnotationVersion, cfg.Version)
	check(internal.AnnotationKyvernoVersion, cfg.KyvernoVersion)
	return warnings
}

// kyvernoLine renders the text layout's "Kyverno:" value, or "" when the config declares no
// constraint, in which case the caller omits the whole line.
func kyvernoLine(c Compatibility) string {
	switch c.Result {
	case "undeclared":
		return ""
	case "satisfied":
		return fmt.Sprintf("%s (satisfied by %s)", c.KyvernoVersion, c.CheckedAgainst)
	case "unsatisfied":
		return fmt.Sprintf("%s (not satisfied by %s)", c.KyvernoVersion, c.CheckedAgainst)
	case "invalid":
		return fmt.Sprintf("%s (invalid constraint)", c.KyvernoVersion)
	default:
		return fmt.Sprintf("%s (not checked; %s is not a release version)", c.KyvernoVersion, c.CheckedAgainst)
	}
}

// sourceLine renders the text layout's "Source:" value as "url @ revision", or just one of them,
// or "" when s is nil or both fields are empty, in which case the caller omits the whole line.
func sourceLine(s *bundle.Source) string {
	if s == nil {
		return ""
	}
	switch {
	case s.URL != "" && s.Revision != "":
		return fmt.Sprintf("%s @ %s", s.URL, s.Revision)
	case s.URL != "":
		return s.URL
	case s.Revision != "":
		return s.Revision
	default:
		return ""
	}
}

// writeText renders the fixed text layout, omitting a line for any empty optional field. Every
// field sourced from the config or manifest annotations — as opposed to the reference/digest the
// caller supplied or passed CheckManifestShape's own fixed media-type strings — is attacker
// input from the registry and goes through sanitize first.
func writeText(w io.Writer, r Report, cfg bundle.Config) error {
	tw := tabwriter.NewWriter(w, 0, 2, 1, ' ', 0)
	fmt.Fprintf(tw, "Reference:\t%s\n", r.Reference)
	fmt.Fprintf(tw, "Digest:\t%s\n", r.Digest)
	fmt.Fprintf(tw, "Pinned:\t%s\n", r.PinnedReference)
	if cfg.FormatVersion != "" {
		fmt.Fprintf(tw, "Format version:\t%s\n", sanitize(cfg.FormatVersion))
	}
	if cfg.Name != "" {
		fmt.Fprintf(tw, "Name:\t%s\n", sanitize(cfg.Name))
	}
	if cfg.Version != "" {
		fmt.Fprintf(tw, "Version:\t%s\n", sanitize(cfg.Version))
	}
	if cfg.Description != "" {
		fmt.Fprintf(tw, "Description:\t%s\n", sanitize(cfg.Description))
	}
	if line := kyvernoLine(r.Compatibility); line != "" {
		fmt.Fprintf(tw, "Kyverno:\t%s\n", sanitize(line))
	}
	if line := sourceLine(cfg.Source); line != "" {
		fmt.Fprintf(tw, "Source:\t%s\n", sanitize(line))
	}
	if cfg.Created != "" {
		fmt.Fprintf(tw, "Created:\t%s\n", sanitize(cfg.Created))
	}
	for _, l := range r.Manifest.Layers {
		fmt.Fprintf(tw, "Layer:\t%s %s (%d bytes)\n", l.MediaType, l.Digest, l.Size)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	if len(cfg.Sets) > 0 {
		if _, err := fmt.Fprintln(w, "Sets:"); err != nil {
			return err
		}
		for _, s := range cfg.Sets {
			if _, err := fmt.Fprintf(w, "  %s (%s)\n", sanitize(s.Name), sanitize(s.Type)); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintf(w, "Resources (%d):\n", len(cfg.Resources)); err != nil {
		return err
	}
	if len(cfg.Resources) > 0 {
		rtw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(rtw, "  APIVERSION\tKIND\tNAMESPACE\tNAME\tSET\tPATH")
		for _, e := range cfg.Resources {
			namespace := e.Namespace
			if namespace == "" {
				namespace = "-"
			}
			path := e.Path
			if e.DocumentIndex != 0 {
				path = fmt.Sprintf("%s (document %d)", e.Path, e.DocumentIndex)
			}
			fmt.Fprintf(rtw, "  %s\t%s\t%s\t%s\t%s\t%s\n", sanitize(e.APIVersion), sanitize(e.Kind), sanitize(namespace), sanitize(e.Name), sanitize(e.Set), sanitize(path))
		}
		if err := rtw.Flush(); err != nil {
			return err
		}
	}

	if len(r.Annotations) == 0 {
		_, err := fmt.Fprintln(w, "Annotations:    none")
		return err
	}
	if _, err := fmt.Fprintln(w, "Annotations:"); err != nil {
		return err
	}
	keys := make([]string, 0, len(r.Annotations))
	for k := range r.Annotations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "  %s: %s\n", sanitize(k), sanitize(r.Annotations[k])); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w io.Writer, r Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report to json: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func writeYAML(w io.Writer, r Report) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}
	out, err := yaml.JSONToYAML(data)
	if err != nil {
		return fmt.Errorf("marshaling report to yaml: %w", err)
	}
	_, err = w.Write(out)
	return err
}
