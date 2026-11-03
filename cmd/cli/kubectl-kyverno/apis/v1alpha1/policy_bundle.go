package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// PolicyBundle is the optional kyverno-bundle.yaml descriptor at the root of a `kyverno oci push`
// input tree. See docs/dev/oci/bundle-spec.md section 7 for the normative shape.
type PolicyBundle struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares the bundle's metadata and named set membership.
	Spec PolicyBundleSpec `json:"spec,omitempty"`
}

// PolicyBundleSpec is the descriptor's spec. Version, Description, FormatVersion, KyvernoVersion,
// and Created are scalar strings; Annotations is a map of strings; Source is a nested object and
// Sets is a list of nested objects, but every leaf scalar inside either of those, and
// metadata.name and metadata.annotations above, is still a string. Quote a value that could
// otherwise be read as a YAML number or boolean (for example "1.20", "yes", "0123456"). Push
// rejects an unquoted one anywhere in this scope instead of letting YAML silently reformat it.
type PolicyBundleSpec struct {
	// Version is the bundle's own version string, passed through to the config blob verbatim.
	Version string `json:"version,omitempty"`

	// Description is a human-readable summary of the bundle, passed through to the config blob.
	Description string `json:"description,omitempty"`

	// FormatVersion pins the bundle format version this descriptor was written against. It must
	// exactly match the format version this CLI writes, or push fails before writing anything.
	FormatVersion string `json:"formatVersion,omitempty"`

	// KyvernoVersion is a semver constraint string (for example ">=1.20") naming the Kyverno
	// versions this bundle targets. Push rejects a malformed one before writing anything; push and
	// `kyverno oci validate` warn when the CLI's own version does not satisfy it.
	KyvernoVersion string `json:"kyvernoVersion,omitempty"`

	// Source records where the bundle's content comes from.
	Source *PolicyBundleSource `json:"source,omitempty"`

	// Created is an RFC 3339 timestamp passed through to the config blob and manifest
	// annotations verbatim.
	Created string `json:"created,omitempty"`

	// Sets declares named sets and the path globs that assign resources to them. A resource not
	// claimed by any set here falls back to the by-kind default set (policies or exceptions).
	Sets []PolicyBundleSet `json:"sets,omitempty"`

	// Annotations are extra manifest annotations. A key under the reserved io.kyverno.bundle. or
	// org.opencontainers.image. prefix here is discarded — the whole prefix is reserved, not only
	// the specific keys a writer currently computes — and the writer-computed value for any key it
	// does compute always wins. Quote a key or value that could otherwise be read as a YAML number
	// or boolean; push rejects either.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PolicyBundleSource records where a bundle's source lives.
type PolicyBundleSource struct {
	// URL is the bundle's source location, for example a Git repository URL.
	URL string `json:"url,omitempty"`

	// Revision is the source revision the bundle was built from, for example a Git commit SHA.
	Revision string `json:"revision,omitempty"`
}

// PolicyBundleSet declares one named set and the path globs that assign resources to it.
type PolicyBundleSet struct {
	// Name is the set's name. The reserved names "policies" and "exceptions" must carry the
	// matching type.
	Name string `json:"name"`

	// Type is the set's type, "policies" or "exceptions". It filters which matched documents the
	// set claims: a "policies" set claims every matched document except PolicyException, and an
	// "exceptions" set claims only matched PolicyException documents.
	Type string `json:"type"`

	// Paths are glob patterns, relative to the bundle root, matched against each archived
	// document's path. "*" and "?" do not cross a "/"; "**" does.
	Paths []string `json:"paths"`
}
