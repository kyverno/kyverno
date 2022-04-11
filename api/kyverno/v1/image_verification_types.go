package v1

import (
	"encoding/json"

	"github.com/pkg/errors"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ImageVerification validates that images that match the specified pattern
// are signed with the supplied public key. Once the image is verified it is
// mutated to include the SHA digest retrieved during the registration.
type ImageVerification struct {

	// Image is the image name consisting of the registry address, repository, image, and tag.
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	// Deprecated. Use ImageReferences instead.
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// ImageReferences is a list of matching image reference patterns. At least one pattern in the
	// list must match the image for the rule to apply. Each image reference consists of a registry
	// address (defaults to docker.io), repository, image, and tag (defaults to latest).
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	// +kubebuilder:default:={"*"}
	ImageReferences []string `json:"imageReferences,omitempty" yaml:"imageReferences,omitempty"`

	// Key is the PEM encoded public key that the image or attestation is signed with.
	// Deprecated. Use StaticKeyAttestor instead.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`

	// Roots is the PEM encoded Root certificate chain used for keyless signing
	// Deprecated. Use KeylessAttestor instead.
	Roots string `json:"roots,omitempty" yaml:"roots,omitempty"`

	// Subject is the identity used for keyless signing, for example an email address
	// Deprecated. Use KeylessAttestor instead.
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`

	// Issuer is the certificate issuer used for keyless signing.
	// Deprecated. Use KeylessAttestor instead.
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`

	// AdditionalExtensions are certificate-extensions used for keyless signing.
	// Deprecated.
	AdditionalExtensions map[string]string `json:"additionalExtensions,omitempty" yaml:"additionalExtensions,omitempty"`

	// Attestors specified the required attestors (i.e. authorities)
	// +kubebuilder:validation:Optional
	Attestors []*AttestorSet `json:"attestors,omitempty" yaml:"attestors,omitempty"`

	// Attestations are optional checks for signed in-toto Statements used to verify the image.
	// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
	// OCI registry and decodes them into a list of Statement declarations.
	Attestations []*Attestation `json:"attestations,omitempty" yaml:"attestations,omitempty"`

	// Annotations are used for image verification.
	// Every specified key-value pair must exist and match in the verified payload.
	// The payload may contain other key-value pairs.
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Repository is an optional alternate OCI repository to use for image signatures that match this rule.
	// If specified Repository will override the default OCI image repository configured for the installation.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
}

type AttestorSet struct {

	// Count specifies the required number of entries that must match. If the count is null, all entries must match
	// (a logical AND). If the count is 1, at least one entry must match (a logical OR). If the count contains a
	// value N, then N must be less than or equal to the size of entries, and at least N entries must match.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum:=1
	Count *int `json:"count,omitempty" yaml:"count,omitempty"`

	// Entries contains the available attestors. An attestor can be a static key,
	// attributes for keyless verification, or a nested attestor declaration.
	// +kubebuilder:validation:Optional
	Entries []*Attestor `json:"entries,omitempty" yaml:"entries,omitempty"`
}

type Attestor struct {

	// StaticKey is a set of attributes used to verify an X.509 public key
	// +kubebuilder:validation:Optional
	StaticKey *StaticKeyAttestor `json:"staticKey,omitempty" yaml:"staticKey,omitempty"`

	// Keyless is a set of attribute used to verify a Sigstore keyless attestor.
	// See https://github.com/sigstore/cosign/blob/main/KEYLESS.md.
	// +kubebuilder:validation:Optional
	Keyless *KeylessAttestor `json:"keyless,omitempty" yaml:"keyless,omitempty"`

	// Attestor is a nested AttestorSet used to specify a more complex set of match authorities
	// +kubebuilder:validation:Optional
	// TODO: Nested typed declarations are cause issues with OpenAPIv3 Schemas. Need to revisit.
	Attestors []*apiextv1.JSON `json:"attestors,omitempty" yaml:"attestors,omitempty"`
}

type StaticKeyAttestor struct {

	// Key is an X.509 public key used to verify image signatures. The key can be directly
	// specified or can be a variable reference to a key specified in a ConfigMap (see
	// https://kyverno.io/docs/writing-policies/variables/).
	Key string `json:"key,omitempty" yaml:"key,omitempty"`
}

type KeylessAttestor struct {

	// Issuer is the certificate issuer used for keyless signing.
	// +kubebuilder:validation:Required
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`

	// Subject is the verified identity used for keyless signing, for example the email address
	// +kubebuilder:validation:Required
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`

	// Roots is a PEM encoded CA certificate chain
	// +kubebuilder:validation:Optional
	Roots string `json:"roots,omitempty" yaml:"roots,omitempty"`

	// AdditionalExtensions are certificate-extensions used for keyless signing.
	// +kubebuilder:validation:Optional
	AdditionalExtensions map[string]string `json:"additionalExtensions,omitempty" yaml:"additionalExtensions,omitempty"`
}

// Attestation are checks for signed in-toto Statements that are used to verify the image.
// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
// OCI registry and decodes them into a list of Statements.
type Attestation struct {

	// PredicateType defines the type of Predicate contained within the Statement.
	PredicateType string `json:"predicateType,omitempty" yaml:"predicateType,omitempty"`

	// Conditions are used to verify attributes within a Predicate. If no Conditions are specified
	// the attestation check is satisfied as long there are predicates that match the predicate type.
	// +optional
	Conditions []*AnyAllConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Validate implements programmatic validation
func (iv *ImageVerification) Validate(path *field.Path) (errs field.ErrorList) {
	if iv.Image == "" && len(iv.ImageReferences) == 0 {
		errs = append(errs, field.Invalid(path, iv, "An image reference is required"))
	}

	hasKey := iv.Key != ""
	hasIssuer := iv.Issuer != ""
	hasSubject := iv.Subject != ""
	hasRoots := iv.Roots != ""
	hasKeyless := hasIssuer || hasSubject || hasRoots
	hasAttestors := len(iv.Attestors) > 0

	if (hasKey && (hasKeyless || hasAttestors)) ||
		(hasKeyless && (hasKey || hasAttestors)) ||
		(hasAttestors && (hasKey || hasKeyless)) ||
		(!hasKey && !hasKeyless && !hasAttestors) {
		errs = append(errs, field.Invalid(path, iv, "Either a static key, keyless, or an attestor is required"))
	}

	if hasKeyless && (!hasIssuer || !hasSubject) {
		errs = append(errs, field.Invalid(path, iv, "An issuer and a subject are required for keyless verification"))
	}

	if len(iv.Attestors) > 1 {
		errs = append(errs, field.Invalid(path, iv, "Only one attestor is currently supported"))
	}

	attestorsPath := path.Child("attestors")
	for i, as := range iv.Attestors {
		attestorErrors := as.Validate(attestorsPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	return errs
}

func (as *AttestorSet) Validate(path *field.Path) (errs field.ErrorList) {
	return validateAttestorSet(as, path)
}

func validateAttestorSet(as *AttestorSet, path *field.Path) (errs field.ErrorList) {
	if as.Count != nil {
		if *as.Count > len(as.Entries) {
			errs = append(errs, field.Invalid(path, as, "Count cannot exceed length of entries"))
		}
	}

	if len(as.Entries) == 0 {
		errs = append(errs, field.Invalid(path, as, "An entry is required"))
	} else if len(as.Entries) > 1 {
		errs = append(errs, field.Invalid(path, as, "Only one entry is currently supported"))
	}

	entriesPath := path.Child("entries")
	for i, e := range as.Entries {
		attestorErrors := e.Validate(entriesPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	return errs
}

func (a *Attestor) Validate(path *field.Path) (errs field.ErrorList) {
	if (a.StaticKey != nil && (a.Keyless != nil || len(a.Attestors) != 0)) ||
		(a.Keyless != nil && (a.StaticKey != nil || len(a.Attestors) != 0)) ||
		(len(a.Attestors) > 0 && (a.StaticKey != nil || a.Keyless != nil)) ||
		(a.StaticKey == nil && a.Keyless == nil && len(a.Attestors) == 0) {
		errs = append(errs, field.Invalid(path, a, "One of static key, keyless, or nested attestor is required"))
	}

	if a.StaticKey != nil {
		staticKeyPath := path.Child("staticKey")
		staticKeyErrors := a.StaticKey.Validate(staticKeyPath)
		errs = append(errs, staticKeyErrors...)
	}

	if a.Keyless != nil {
		keylessPath := path.Child("keyless")
		keylessErrors := a.Keyless.Validate(keylessPath)
		errs = append(errs, keylessErrors...)
	}

	attestorsPath := path.Child("attestors")
	for i, rawJson := range a.Attestors {
		attestorSet, err := AttestorSetUnmarshal(rawJson)
		if err != nil {
			fieldErr := field.Invalid(attestorsPath.Index(i), rawJson, err.Error())
			errs = append(errs, fieldErr)
			continue
		}

		attestorErrors := validateAttestorSet(attestorSet, attestorsPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	return errs
}

func AttestorSetUnmarshal(o *apiextv1.JSON) (*AttestorSet, error) {
	var as AttestorSet
	if err := json.Unmarshal(o.Raw, &as); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal attestor set %s", string(o.Raw))
	}

	return &as, nil
}

func (ska *StaticKeyAttestor) Validate(path *field.Path) (errs field.ErrorList) {
	if ska.Key == "" {
		errs = append(errs, field.Invalid(path, ska, "A key is required"))
	}

	return errs
}

func (ka *KeylessAttestor) Validate(path *field.Path) (errs field.ErrorList) {
	if ka.Issuer == "" {
		errs = append(errs, field.Invalid(path, ka, "An issuer is required"))
	}

	if ka.Subject == "" {
		errs = append(errs, field.Invalid(path, ka, "A subject is required"))
	}

	return errs
}

func (iv *ImageVerification) Convert() *ImageVerification {
	if len(iv.ImageReferences) > 0 || len(iv.Attestors) > 0 {
		return iv
	}

	copy := &ImageVerification{}
	copy.Attestations = iv.Attestations

	if iv.Image != "" {
		copy.ImageReferences = []string{iv.Image}
	}

	var attestor Attestor
	if iv.Key != "" {
		attestor.StaticKey = &StaticKeyAttestor{
			Key: iv.Key,
		}
	} else if iv.Issuer != "" {
		attestor.Keyless = &KeylessAttestor{
			Issuer:  iv.Issuer,
			Subject: iv.Subject,
			Roots:   iv.Roots,
		}
	}

	attestorSet := &AttestorSet{}
	attestorSet.Entries = append(attestorSet.Entries, &attestor)

	copy.Attestors = append(copy.Attestors, attestorSet)
	return copy
}
