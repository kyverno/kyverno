package internal

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Format 1.0 media types, per docs/dev/oci/bundle-spec.md section 4.
const (
	// ContentMediaType is the single tar+gzip content layer's media type.
	ContentMediaType = "application/vnd.cncf.kyverno.bundle.content.v1.tar+gzip"
	// ConfigMediaType is the config blob's media type.
	ConfigMediaType = "application/vnd.cncf.kyverno.bundle.config.v1+json"

	// LegacyConfigMediaType and LegacyContentMediaType identify a pre-1.20 image so a reader
	// can fail with an actionable "legacy image format" error instead of a confusing one.
	LegacyConfigMediaType  = "application/vnd.cncf.kyverno.config.v1+json"
	LegacyContentMediaType = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
)

// Manifest annotation keys, per docs/dev/oci/bundle-spec.md section 4.
const (
	AnnotationFormatVersion  = "io.kyverno.bundle.format-version"
	AnnotationName           = "io.kyverno.bundle.name"
	AnnotationVersion        = "io.kyverno.bundle.version"
	AnnotationKyvernoVersion = "io.kyverno.bundle.kyverno-version"

	AnnotationOCISource      = "org.opencontainers.image.source"
	AnnotationOCIRevision    = "org.opencontainers.image.revision"
	AnnotationOCICreated     = "org.opencontainers.image.created"
	AnnotationOCIVersion     = "org.opencontainers.image.version"
	AnnotationOCITitle       = "org.opencontainers.image.title"
	AnnotationOCIDescription = "org.opencontainers.image.description"
)

// FormatVersion is this format's version, carried in the config's formatVersion field, the
// io.kyverno.bundle.format-version annotation, and the media types' .v1 segment (major only).
const FormatVersion = "1.0.0"

// DescriptorFilename is the reserved, optional author-written bundle descriptor. It's never
// archived; see docs/dev/oci/bundle-spec.md section 5 and 7.
const DescriptorFilename = "kyverno-bundle.yaml"

// Object is satisfied by all CEL policy kinds and CEL PolicyException which
// embed metav1.ObjectMeta and metav1.TypeMeta so that GroupVersionKind is
// available via GetObjectKind().
type Object interface {
	metav1.Object
	runtime.Object
}
