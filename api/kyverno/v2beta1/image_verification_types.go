package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ImageVerification validates that images that match the specified pattern
// are signed with the supplied public key. Once the image is verified it is
// mutated to include the SHA digest retrieved during the registration.
type ImageVerification struct {
	// ImageReferences is a list of matching image reference patterns. At least one pattern in the
	// list must match the image for the rule to apply. Each image reference consists of a registry
	// address (defaults to docker.io), repository, image, and tag (defaults to latest).
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	// +kubebuilder:validation:Optional
	ImageReferences []string `json:"imageReferences,omitempty" yaml:"imageReferences,omitempty"`

	// Attestors specified the required attestors (i.e. authorities)
	// +kubebuilder:validation:Optional
	Attestors []kyvernov1.AttestorSet `json:"attestors,omitempty" yaml:"attestors,omitempty"`

	// Attestations are optional checks for signed in-toto Statements used to verify the image.
	// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
	// OCI registry and decodes them into a list of Statement declarations.
	Attestations []kyvernov1.Attestation `json:"attestations,omitempty" yaml:"attestations,omitempty"`

	// Repository is an optional alternate OCI repository to use for image signatures and attestations that match this rule.
	// If specified Repository will override the default OCI image repository configured for the installation.
	// The repository can also be overridden per Attestor or Attestation.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`

	// MutateDigest enables replacement of image tags with digests.
	// Defaults to true.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	MutateDigest bool `json:"mutateDigest" yaml:"mutateDigest"`

	// VerifyDigest validates that images have a digest.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	VerifyDigest bool `json:"verifyDigest" yaml:"verifyDigest"`

	// Required validates that images are verified i.e. have matched passed a signature or attestation check.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	Required bool `json:"required" yaml:"required"`
}

// Validate implements programmatic validation
func (iv *ImageVerification) Validate(path *field.Path) (errs field.ErrorList) {
	copy := iv

	if len(copy.ImageReferences) == 0 {
		errs = append(errs, field.Invalid(path, iv, "An image reference is required"))
	}

	hasAttestors := len(copy.Attestors) > 0
	hasAttestations := len(copy.Attestations) > 0

	if hasAttestations && !hasAttestors {
		errs = append(errs, field.Invalid(path, iv, "An attestor is required"))
	}

	attestorsPath := path.Child("attestors")
	for i, as := range copy.Attestors {
		attestorErrors := as.Validate(attestorsPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	return errs
}
