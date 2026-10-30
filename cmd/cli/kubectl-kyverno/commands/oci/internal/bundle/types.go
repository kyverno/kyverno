// Package bundle implements the format 1.0 Kyverno CEL Policy Bundle pipeline described in
// docs/dev/oci/bundle-spec.md: Assemble walks an input tree into a Bundle, Validate checks it,
// Write serializes it to a v1.Image, and Read does the reverse. See the spec for the normative
// rules; this package is the implementation of them.
//
// docs/dev/oci/bundle-spec.md lands with #17660, a sibling of this package's own issue (#17662),
// and doesn't exist yet on this branch or on upstream/main as of this package's first commit.
// Every "bundle-spec.md section N" citation in this package is a forward reference to that file;
// treat it as the intended target the citations describe, not as something you can currently
// open in this repository, until #17660 merges.
package bundle

import "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"

// Document is one non-empty YAML document found while assembling a bundle, identified the way
// the spec's resource-index entry is (bundle-spec.md section 6, "Resource-index entry").
type Document struct {
	// Path is the archived file's path relative to the bundle root, forward-slash separated.
	Path string
	// Index is the document's zero-based position among the non-empty documents the reference
	// splitter (ext/yaml.SplitDocuments) returns for Path.
	Index int
	// Bytes are the document's bytes exactly as the reference splitter returns them: the digest
	// input, not necessarily identical to a byte range sliced directly out of the archived file.
	Bytes []byte
	// Digest is "sha256:" followed by the hex digest of Bytes.
	Digest string

	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// Bundle is the in-memory result of Assemble: the raw per-document index data needed to write
// the content layer and the config's resource index, plus the typed objects Validate and Write
// need for CEL compilation, exception-reference resolution, and default set assignment.
type Bundle struct {
	// Documents holds one entry per non-empty document found under the bundle root, in the
	// order Assemble encountered them.
	Documents []Document
	// Files holds each archived file's original bytes, keyed by its bundle-root-relative,
	// forward-slash path. These are the bytes the content layer archives; they are not
	// necessarily identical to any single Document's Bytes for a multi-document file.
	Files map[string][]byte
	// Results is the typed load of every archived file, in the shape push and pull already use
	// for CEL compilation and exception-reference resolution.
	Results *policy.LoaderResults
}

// Source records where a bundle's source lives, mirroring the descriptor's spec.source and the
// config blob's source field (bundle-spec.md section 6).
type Source struct {
	URL      string `json:"url,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// Descriptor is the normalized form of the optional kyverno-bundle.yaml input that Write
// serializes into the config blob and manifest annotations. Parsing kyverno-bundle.yaml into a
// Descriptor, and deriving Sets from spec.sets[].paths, is #17663's job; a nil Descriptor is a
// push with no descriptor, and Write applies the no-descriptor defaults from bundle-spec.md
// section 6 in that case (omit every optional field, assign Sets by kind).
type Descriptor struct {
	Name           string
	Version        string
	Description    string
	KyvernoVersion string
	Source         *Source
	// Created is the config's created field and the org.opencontainers.image.created
	// annotation. Both are omitted unless this is set.
	Created *string
	// Sets overrides the by-kind default set assignment. A nil or empty Sets falls back to the
	// two default sets, policies and exceptions, assigned by kind.
	Sets []SetAssignment
	// Annotations are extra manifest annotations from the descriptor's spec.annotations. A
	// reserved io.kyverno.bundle.* or org.opencontainers.image.* key here is ignored; the
	// writer-computed value for a reserved key always wins (bundle-spec.md section 4).
	Annotations map[string]string
}

// SetAssignment assigns one resource, identified by Kind/namespace/name, to a named set.
type SetAssignment struct {
	Kind      string
	Namespace string
	Name      string
	Set       string
	// Type is the set's type, "policies" or "exceptions".
	Type string
}

// Config is the config blob's JSON schema (bundle-spec.md section 6).
type Config struct {
	FormatVersion  string          `json:"formatVersion"`
	Name           string          `json:"name,omitempty"`
	Version        string          `json:"version,omitempty"`
	Description    string          `json:"description,omitempty"`
	KyvernoVersion string          `json:"kyvernoVersion,omitempty"`
	Source         *Source         `json:"source,omitempty"`
	Created        string          `json:"created,omitempty"`
	Sets           []ConfigSet     `json:"sets"`
	Resources      []ResourceEntry `json:"resources"`
}

// ConfigSet is the config blob's set object (bundle-spec.md section 6, "Set object").
type ConfigSet struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ResourceEntry is one config blob resource-index entry (bundle-spec.md section 6,
// "Resource-index entry").
type ResourceEntry struct {
	APIVersion    string `json:"apiVersion"`
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DocumentIndex int    `json:"documentIndex"`
	Digest        string `json:"digest"`
	Set           string `json:"set"`
}
