package bundle

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
)

// Read is the format 1.0 reader: it requires the exact manifest shape (bundle-spec.md section 4,
// reader MUST 1), extracts the content layer under the archive path rules (reader MUST 2),
// re-runs ValidateForRead over the extracted tree (reader MUST 3), and cross-checks every
// extracted document against the config's resource index by path, documentIndex, digest,
// apiVersion, kind, namespace, and name, and confirms every index entry's set names an entry in
// the config's own sets[] (reader MUST 4). Any mismatch is a hard failure: the registry is
// writable by anyone with push rights and by tools other than kyverno oci push, so a config that
// disagrees with the archive is a tamper or tooling signal, not a nuisance to ignore.
//
// Extraction and validation both happen in a private staging directory, not dir, for two
// reasons. First, dir may already hold unrelated files (a previous pull, a directory an author
// also edits by hand); Assemble must never see them, or a pre-existing YAML file in dir would be
// validated and cross-checked as if it were part of this bundle and fail the pull. Second, and
// more importantly, extract writes every regular tar entry unconditionally — it has to, to stay
// a dumb, format-agnostic archive reader — while Assemble silently excludes whole classes of
// path (hidden files, non-.yaml/.yml files, the reserved descriptor at the bundle root) the same
// way it does on the write side. Only the entries that survive Assemble, i.e. only the archived
// files that ended up in b.Files, are ever placed into dir, by place below. A tampered or
// malicious archive can add a root kyverno-bundle.yaml, a hidden file, or a non-YAML file
// alongside legitimate, indexed content; those extra entries reach the staging directory during
// extract but are never copied into dir, so they can never reach a caller such as `kyverno apply
// oci://` or `kyverno test oci://` that loads dir afterward. What's true as a result: every
// *document* that ends up on disk after Read is indexed and validated. A .yaml/.yml file whose
// only documents are empty (blank lines or #-comment lines) is still placed — it's archived
// content per bundle-spec.md section 5 — but carries no resources[] entry and nothing to
// validate, the same as it doesn't on the write side.
//
// Read uses ValidateForRead, not Validate: exception-to-policy reference resolution is push's
// own strict default, not a format requirement (bundle-spec.md section 6, "Sets"), so a bundle
// whose PolicyException references a policy outside the bundle is format-legal and must
// round-trip here.
//
// (bundle-spec.md lands with #17660; until that merges, the section references throughout this
// package are forward citations to a file that doesn't exist yet on this branch or on
// upstream/main.)
func Read(img v1.Image, dir string) (*Bundle, error) {
	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	if err := checkManifestShape(manifest); err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("getting image layers: %w", err)
	}

	staging, err := os.MkdirTemp("", "kyverno-oci-read-*")
	if err != nil {
		return nil, fmt.Errorf("creating staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	// Assemble's own error messages embed the root path it was given, which is the right thing
	// for a push (where the author needs to know which of their own directories is the problem)
	// and the wrong thing here: it's our private, disposable staging directory, and a user
	// pulling a bundle has no reason to see a temp path like
	// /var/folders/.../kyverno-oci-read-123 in an error. resolvedStaging accounts for
	// Assemble's own filepath.EvalSymlinks call, which can rewrite the path it reports (for
	// example macOS's /var -> /private/var) to something that no longer matches staging
	// byte-for-byte.
	resolvedStaging, evalErr := filepath.EvalSymlinks(staging)
	if evalErr != nil {
		resolvedStaging = staging
	}

	if err := extract(layers[0], staging); err != nil {
		return nil, err
	}

	b, err := Assemble(staging)
	if err != nil {
		return nil, fmt.Errorf("re-validating extracted bundle: %w", scrubPath(err, staging, resolvedStaging))
	}
	if err := ValidateForRead(b); err != nil {
		return nil, err
	}

	configBytes, err := img.RawConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Reader MUST 6: reject a bundle whose major format version this reader doesn't support,
	// naming both versions. This is checked here, against the parsed config, rather than folded
	// into checkManifestShape, because the version lives in the config, not the manifest.
	if err := checkFormatVersion(cfg.FormatVersion); err != nil {
		return nil, err
	}

	if err := crossCheckIndex(b.Documents, cfg.Resources, cfg.Sets); err != nil {
		return nil, err
	}

	// Only now, with every check passed, place the validated content at dir. b.Files holds
	// exactly the archived files Assemble accepted; anything extract wrote to staging that
	// Assemble excluded (hidden paths, non-YAML files, a root kyverno-bundle.yaml) is discarded
	// with staging above and never reaches dir. dir's own pre-existing contents, if any, are
	// left alone except at paths this bundle actually supplies.
	if err := place(b.Files, dir); err != nil {
		return nil, err
	}

	return b, nil
}

// scrubPath removes every occurrence of paths from err's message and collapses the resulting
// run of whitespace, so an internal filesystem path (most commonly Read's own disposable
// staging directory) doesn't leak into an error a caller sees.
func scrubPath(err error, paths ...string) error {
	msg := err.Error()
	for _, p := range paths {
		if p == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, p, "")
	}
	return errors.New(strings.Join(strings.Fields(msg), " "))
}

// supportedFormatMajor is the bundle format major version this reader supports (bundle-spec.md
// section 8, "Compatibility guarantee": Kyverno 1.20 supports major version 1).
const supportedFormatMajor = "1"

// checkFormatVersion rejects a bundle whose major format version this reader doesn't support,
// naming both versions (reader MUST 6). formatVersion must be a full semver string: exactly three
// dot-separated, all-numeric components (major.minor.patch, for example "1.0.0"). Anything else —
// empty, missing a component, a non-numeric component, or a trailing/leading empty component — is
// malformed and rejected before the major-version comparison even runs: the config is the
// writer's own claim about the format it used, and a reader that can't parse that claim can't
// safely read the rest of it either. This deliberately rejects a semver pre-release or build
// suffix (for example "1.0.0-rc.1"), matching bundle-spec.md section 8's "spec.formatVersion's pin
// is a full semver string ... and MUST match, exactly, a version string in the writer's own list
// of supported format versions" — this format's formatVersion values are plain major.minor.patch,
// not general semver.
func checkFormatVersion(formatVersion string) error {
	malformed := fmt.Errorf("malformed bundle: config formatVersion %q is not a valid semver string", formatVersion)

	parts := strings.Split(formatVersion, ".")
	if len(parts) != 3 {
		return malformed
	}
	for _, p := range parts {
		if p == "" {
			return malformed
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return malformed
			}
		}
	}

	major := parts[0]
	if major != supportedFormatMajor {
		return fmt.Errorf("unsupported bundle format major version %s (this reader supports major version %s)", major, supportedFormatMajor)
	}
	return nil
}

// place writes files (bundle-root-relative path -> validated content) under dir, refusing any
// path that would escape it. It runs a pre-flight pass over every target before writing anything,
// so a problem with one file (most commonly, a pre-existing directory or symlink already sitting
// at that path) fails the whole call before any file is written, rather than leaving some of the
// bundle's files placed and others missing. A pre-existing symlink anywhere along a target's
// path — not only at its final component, but at any intermediate directory component too — is
// refused outright rather than written through: securejoin.SecureJoin confines the eventual
// write inside dir even if a symlink's target is absolute or points outside dir, which is the
// right safety property, but silently relocating the write to wherever that confinement lands
// (nested many directories deep, mirroring the symlink target's own path) while still reporting
// success would break the round-trip guarantee just as badly as an escape would, without ever
// telling the caller. checkPlacementPath below is what walks every component, not just the leaf:
// a symlinked directory (dst/policies -> /tmp/outside) is exactly as dangerous here as a
// symlinked leaf file, and only checking the leaf would miss it.
func place(files map[string][]byte, dir string) error {
	targets := make(map[string]string, len(files))
	for rel := range files {
		if err := checkPlacementPath(dir, rel); err != nil {
			return err
		}
		target, err := securejoin.SecureJoin(dir, rel)
		if err != nil {
			return fmt.Errorf("constructing output path for %q: %w", rel, err)
		}
		targets[rel] = target
	}

	for rel, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("creating directory for %q: %w", rel, err)
		}
		if err := os.WriteFile(target, files[rel], 0o600); err != nil {
			return fmt.Errorf("writing %q: %w", rel, err)
		}
	}
	return nil
}

// checkPlacementPath walks rel's path components under dir, literally (not through
// securejoin's symlink-confining resolution), and refuses if any existing component — including
// an intermediate directory, not only the final file — is a symlink, or if a non-final component
// exists but isn't a directory, or if the final component already exists as a directory. A
// component that doesn't exist yet (os.ErrNotExist) is fine: MkdirAll and WriteFile create it
// normally below. Any other Lstat error — most notably ENOTDIR, which is what a *later*
// component's Lstat returns when an *earlier* component turned out to be a regular file, not a
// directory — is treated as a refusal, not as absence: the earlier, buggy version of this
// function only checked whether the leaf itself was a directory, so a plain file sitting at an
// intermediate path (for example dir/policies as a regular file, with the bundle wanting to
// write dir/policies/a.yaml) passed this check, and the failure only surfaced later, mid-way
// through the write loop below, as a MkdirAll error — which is exactly the partial-output bug
// this function exists to prevent.
func checkPlacementPath(dir, rel string) error {
	parts := strings.Split(rel, "/")
	cur := dir
	for i, p := range parts {
		cur = filepath.Join(cur, p)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("cannot write %q: checking %s: %w", rel, cur, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot write %q: %s already exists as a symlink", rel, cur)
		}
		isLast := i == len(parts)-1
		if isLast {
			if fi.IsDir() {
				return fmt.Errorf("cannot write %q: %s already exists as a directory", rel, cur)
			}
		} else if !fi.IsDir() {
			return fmt.Errorf("cannot write %q: %s already exists and is not a directory", rel, cur)
		}
	}
	return nil
}

// checkManifestShape requires an OCI 1.0 image manifest (schemaVersion 2, mediaType
// application/vnd.oci.image.manifest.v1+json) with exactly one layer, at index 0, with the
// content media type, and a config with the config media type (reader MUST 1). It detects a
// pre-1.20 image by its retired media types and fails with an actionable "legacy image format"
// error instead of a confusing downstream one (bundle-spec.md section 8). The legacy-media-type
// checks run first, and the schemaVersion/mediaType check last, so a pre-1.20 image — which never
// carried either the current config or layer media type, but also never set the OCI manifest
// media type this format now requires — still gets the actionable legacy message instead of a
// generic manifest-shape error.
func checkManifestShape(manifest *v1.Manifest) error {
	if manifest.Config.MediaType == internal.LegacyConfigMediaType {
		return fmt.Errorf("legacy image format: this image predates format 1.0 (config media type %s); republish it with a 1.20 or later kyverno CLI", manifest.Config.MediaType)
	}
	if manifest.Config.MediaType != internal.ConfigMediaType {
		return fmt.Errorf("malformed bundle: expected config media type %s, got %s", internal.ConfigMediaType, manifest.Config.MediaType)
	}

	if len(manifest.Layers) != 1 {
		return fmt.Errorf("malformed bundle: expected exactly one layer, got %d", len(manifest.Layers))
	}
	layerMediaType := manifest.Layers[0].MediaType
	if layerMediaType == internal.LegacyContentMediaType {
		return fmt.Errorf("legacy image format: this image predates format 1.0 (layer media type %s); republish it with a 1.20 or later kyverno CLI", layerMediaType)
	}
	if string(layerMediaType) != internal.ContentMediaType {
		return fmt.Errorf("malformed bundle: expected layer media type %s at index 0, got %s", internal.ContentMediaType, layerMediaType)
	}

	if manifest.SchemaVersion != 2 || manifest.MediaType != types.OCIManifestSchema1 {
		return fmt.Errorf("malformed bundle: expected an OCI 1.0 image manifest (schemaVersion 2, mediaType %s), got schemaVersion %d, mediaType %q", types.OCIManifestSchema1, manifest.SchemaVersion, manifest.MediaType)
	}

	return nil
}

// maxEntryBytes bounds a single content-layer entry's decompressed size (bundle-spec.md section
// 5, "Archive rules"). A CEL policy bundle's largest legitimate entry is one YAML manifest; 10
// MiB is generous headroom over any realistic policy or exception document while still bounding
// a gzip decompression bomb, where the on-wire (compressed) size gives no guarantee about the
// decompressed size.
const maxEntryBytes = 10 << 20 // 10 MiB

// maxTotalBytes bounds the content layer's total decompressed size across all entries
// (bundle-spec.md section 5, "Archive rules"). 100 MiB comfortably fits a bundle of thousands of
// policies while still bounding the layer as a whole against a compressed blob designed to
// expand far beyond it.
const maxTotalBytes = 100 << 20 // 100 MiB

// maxEntries bounds the number of tar entries a content layer may contain, independent of their
// size, so a flood of many small or empty entries can't exhaust memory or inodes either
// (bundle-spec.md section 5, "Archive rules").
const maxEntries = 10000

// extract writes layer's tar+gzip content at their original relative paths under dir, refusing
// any entry that would escape it (bundle-spec.md section 5, "Archive rules"). It bounds
// decompression against a gzip bomb: maxEntryBytes per entry, maxTotalBytes across the whole
// layer, and maxEntries entries, all enforced with io.LimitReader before bytes are accumulated in
// memory rather than after a full, unbounded read.
func extract(layer v1.Layer, dir string) error {
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("getting layer blob: %w", err)
	}
	defer blob.Close()

	gz, err := gzip.NewReader(blob)
	if err != nil {
		return fmt.Errorf("decompressing content layer: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var totalBytes int64
	var entryCount int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading content layer: %w", err)
		}
		entryCount++
		if entryCount > maxEntries {
			return fmt.Errorf("content layer exceeds the maximum of %d entries", maxEntries)
		}
		if hdr.Typeflag != tar.TypeReg {
			return fmt.Errorf("content layer entry %q is not a regular file", hdr.Name)
		}
		name := filepath.ToSlash(hdr.Name)
		if filepath.IsAbs(name) {
			return fmt.Errorf("content layer entry %q has an absolute path", hdr.Name)
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == ".." {
				return fmt.Errorf("content layer entry %q escapes the bundle root", hdr.Name)
			}
		}
		if hdr.Size > maxEntryBytes {
			return fmt.Errorf("content layer entry %q exceeds the maximum entry size of %d bytes", hdr.Name, maxEntryBytes)
		}

		target, err := securejoin.SecureJoin(dir, name)
		if err != nil {
			return fmt.Errorf("constructing output path for %q: %w", hdr.Name, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("creating directory for %q: %w", hdr.Name, err)
		}
		// #nosec G110 -- bounded below: content is read through io.LimitReader capped at
		// maxEntryBytes+1 and rejected before being written or accumulated toward maxTotalBytes,
		// so a tampered or malicious archive can no longer expand an entry (or the layer as a
		// whole) without bound. This replaces the previous unbounded io.ReadAll(tr), carried over
		// unchanged from pull/options.go's io.ReadAll(blob) on main.
		content, err := io.ReadAll(io.LimitReader(tr, maxEntryBytes+1))
		if err != nil {
			return fmt.Errorf("reading content for %q: %w", hdr.Name, err)
		}
		if int64(len(content)) > maxEntryBytes {
			return fmt.Errorf("content layer entry %q exceeds the maximum entry size of %d bytes", hdr.Name, maxEntryBytes)
		}
		totalBytes += int64(len(content))
		if totalBytes > maxTotalBytes {
			return fmt.Errorf("content layer exceeds the maximum total decompressed size of %d bytes at entry %q", maxTotalBytes, hdr.Name)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return fmt.Errorf("writing %q: %w", hdr.Name, err)
		}
	}
	return nil
}

// crossCheckIndex verifies every archived document has exactly one index entry whose path,
// documentIndex, digest, apiVersion, kind, namespace, and name all match, and every index entry
// resolves to an archived document (reader MUST 4). Checking only the digest would let a config
// index claim a different apiVersion, kind, namespace, or name than the archived document while
// still passing, which would mislead any index consumer that trusts these fields (for example,
// `kyverno oci inspect`) without re-parsing the archive itself. sets is the config's own sets[];
// every resource-index entry's set MUST name one of them (bundle-spec.md section 6,
// "Resource-index entry": "MUST name an entry in sets[]"), so an entry naming a set that doesn't
// exist breaks the index contract for any consumer that trusts it, the same way a mismatched
// identity field would. This is checked while building byKey, not while walking documents, so an
// index entry with a bad set is rejected even for a set that matches no archived document at all.
func crossCheckIndex(documents []Document, resources []ResourceEntry, sets []ConfigSet) error {
	setNames := make(map[string]bool, len(sets))
	for _, s := range sets {
		setNames[s.Name] = true
	}

	byKey := make(map[string]ResourceEntry, len(resources))
	for _, r := range resources {
		key := fmt.Sprintf("%s#%d", r.Path, r.DocumentIndex)
		if _, dup := byKey[key]; dup {
			return fmt.Errorf("malformed bundle: config index has more than one entry for %s document %d", r.Path, r.DocumentIndex)
		}
		if r.Set == "" || !setNames[r.Set] {
			return fmt.Errorf("malformed bundle: config index entry %s (document %d) names set %q, which is not in the config's sets[]", r.Path, r.DocumentIndex, r.Set)
		}
		byKey[key] = r
	}

	seen := make(map[string]bool, len(documents))
	for _, doc := range documents {
		key := fmt.Sprintf("%s#%d", doc.Path, doc.Index)
		entry, ok := byKey[key]
		if !ok {
			return fmt.Errorf("malformed bundle: archived document %s (document %d) has no matching config index entry", doc.Path, doc.Index)
		}
		if entry.Digest != doc.Digest {
			return fmt.Errorf("malformed bundle: config index digest for %s (document %d) does not match the archived document", doc.Path, doc.Index)
		}
		if entry.APIVersion != doc.APIVersion {
			return fmt.Errorf("malformed bundle: config index apiVersion for %s (document %d) does not match the archived document", doc.Path, doc.Index)
		}
		if entry.Kind != doc.Kind {
			return fmt.Errorf("malformed bundle: config index kind for %s (document %d) does not match the archived document", doc.Path, doc.Index)
		}
		if entry.Namespace != doc.Namespace {
			return fmt.Errorf("malformed bundle: config index namespace for %s (document %d) does not match the archived document", doc.Path, doc.Index)
		}
		if entry.Name != doc.Name {
			return fmt.Errorf("malformed bundle: config index name for %s (document %d) does not match the archived document", doc.Path, doc.Index)
		}
		seen[key] = true
	}

	for key, entry := range byKey {
		if !seen[key] {
			return fmt.Errorf("malformed bundle: config index entry %s (document %d) has no matching archived document", entry.Path, entry.DocumentIndex)
		}
	}

	return nil
}
