package v1

import "k8s.io/apimachinery/pkg/util/validation/field"

// ImageVerification validates that images that match the specified pattern
// are signed with the supplied public key. Once the image is verified it is
// mutated to include the SHA digest retrieved during the registration.
type ImageVerification struct {

	// Image is the image name consisting of the registry address, repository, image, and tag.
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Key is the PEM encoded public key that the image or attestation is signed with.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`

	// Roots is the PEM encoded Root certificate chain used for keyless signing
	Roots string `json:"roots,omitempty" yaml:"roots,omitempty"`

	// Subject is the verified identity used for keyless signing, for example the email address
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`

	// Issuer is the certificate issuer used for keyless signing.
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`

	// Annotations are used for image verification.
	// Every specified key-value pair must exist and match in the verified payload.
	// The payload may contain other key-value pairs.
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Repository is an optional alternate OCI repository to use for image signatures that match this rule.
	// If specified Repository will override the default OCI image repository configured for the installation.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`

	// Attestations are optional checks for signed in-toto Statements used to verify the image.
	// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
	// OCI registry and decodes them into a list of Statement declarations.
	Attestations []*Attestation `json:"attestations,omitempty" yaml:"attestations,omitempty"`
}

// Validate implements programmatic validation
func (i *ImageVerification) Validate(path *field.Path) field.ErrorList {
	var errs field.ErrorList
	hasKey := i.Key != ""
	hasRoots := i.Roots != ""
	hasSubject := i.Subject != ""
	if (hasKey && !hasRoots && !hasSubject) || (hasRoots && hasSubject) {
		return nil
	}
	errs = append(errs, field.Invalid(path, i, "Either a public key, or root certificates and an email, are required"))
	return errs
}
