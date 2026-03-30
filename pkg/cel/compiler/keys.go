package compiler

const (
	AttestationsKey    = "attestations"
	AttestorsKey       = "attestors"
	GlobalContextKey   = "globalContext"
	HttpKey            = "http"
	ImageDataKey       = "image"
	ImageRefKey        = "ref"
	ImagesKey          = "images"
	NamespaceObjectKey = "namespaceObject"
	ObjectKey          = "object"
	OldObjectKey       = "oldObject"
	RequestKey         = "request"
	ResourceKey        = "resource"
	GeneratorKey       = "generator"
	VariablesKey       = "variables"
	ExceptionsKey      = "exceptions"
	// ImageKey is the variable name for the image reference string available
	// in identity CEL expression evaluation contexts (subject, subjectRegExp).
	// It intentionally reuses the same string value as ImageDataKey so that
	// the identity env and the main policy env use the same variable name.
	ImageKey = ImageDataKey
)
