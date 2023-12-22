package v1

import (
	"encoding/json"
	"fmt"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ImageVerificationType selects the type of verification algorithm
// +kubebuilder:validation:Enum=Cosign;Notary
// +kubebuilder:default=Cosign
type ImageVerificationType string

// ImageRegistryCredentialsProvidersType provides the list of credential providers required.
// +kubebuilder:validation:Enum=default;amazon;azure;google;github
type ImageRegistryCredentialsProvidersType string

const (
	Cosign ImageVerificationType = "Cosign"
	Notary ImageVerificationType = "Notary"

	DEFAULT ImageRegistryCredentialsProvidersType = "default"
	AWS     ImageRegistryCredentialsProvidersType = "amazon"
	ACR     ImageRegistryCredentialsProvidersType = "azure"
	GCP     ImageRegistryCredentialsProvidersType = "google"
	GHCR    ImageRegistryCredentialsProvidersType = "github"
)

var signatureAlgorithmMap = map[string]bool{
	"":       true,
	"sha224": true,
	"sha256": true,
	"sha384": true,
	"sha512": true,
}

// ImageVerification validates that images that match the specified pattern
// are signed with the supplied public key. Once the image is verified it is
// mutated to include the SHA digest retrieved during the registration.
type ImageVerification struct {
	// Type specifies the method of signature validation. The allowed options
	// are Cosign and Notary. By default Cosign is used if a type is not specified.
	// +kubebuilder:validation:Optional
	Type ImageVerificationType `json:"type,omitempty" yaml:"type,omitempty"`

	// Deprecated. Use ImageReferences instead.
	// +kubebuilder:validation:Optional
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// ImageReferences is a list of matching image reference patterns. At least one pattern in the
	// list must match the image for the rule to apply. Each image reference consists of a registry
	// address (defaults to docker.io), repository, image, and tag (defaults to latest).
	// Wildcards ('*' and '?') are allowed. See: https://kubernetes.io/docs/concepts/containers/images.
	// +kubebuilder:validation:Optional
	ImageReferences []string `json:"imageReferences,omitempty" yaml:"imageReferences,omitempty"`

	// Deprecated. Use StaticKeyAttestor instead.
	Key string `json:"key,omitempty" yaml:"key,omitempty"`

	// Deprecated. Use KeylessAttestor instead.
	Roots string `json:"roots,omitempty" yaml:"roots,omitempty"`

	// Deprecated. Use KeylessAttestor instead.
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`

	// Deprecated. Use KeylessAttestor instead.
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`

	// Deprecated.
	AdditionalExtensions map[string]string `json:"additionalExtensions,omitempty" yaml:"additionalExtensions,omitempty"`

	// Attestors specified the required attestors (i.e. authorities)
	// +kubebuilder:validation:Optional
	Attestors []AttestorSet `json:"attestors,omitempty" yaml:"attestors,omitempty"`

	// Attestations are optional checks for signed in-toto Statements used to verify the image.
	// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
	// OCI registry and decodes them into a list of Statement declarations.
	Attestations []Attestation `json:"attestations,omitempty" yaml:"attestations,omitempty"`

	// Deprecated. Use annotations per Attestor instead.
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

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

	// ImageRegistryCredentials provides credentials that will be used for authentication with registry.
	// +kubebuilder:validation:Optional
	ImageRegistryCredentials *ImageRegistryCredentials `json:"imageRegistryCredentials,omitempty" yaml:"imageRegistryCredentials,omitempty"`

	// UseCache enables caching of image verify responses for this rule.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	UseCache bool `json:"useCache" yaml:"useCache"`
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
	Entries []Attestor `json:"entries,omitempty" yaml:"entries,omitempty"`
}

func (as AttestorSet) RequiredCount() int {
	if as.Count == nil || *as.Count == 0 {
		return len(as.Entries)
	}
	return *as.Count
}

type Attestor struct {
	// Keys specifies one or more public keys.
	// +kubebuilder:validation:Optional
	Keys *StaticKeyAttestor `json:"keys,omitempty" yaml:"keys,omitempty"`

	// Certificates specifies one or more certificates.
	// +kubebuilder:validation:Optional
	Certificates *CertificateAttestor `json:"certificates,omitempty" yaml:"certificates,omitempty"`

	// Keyless is a set of attribute used to verify a Sigstore keyless attestor.
	// See https://github.com/sigstore/cosign/blob/main/KEYLESS.md.
	// +kubebuilder:validation:Optional
	Keyless *KeylessAttestor `json:"keyless,omitempty" yaml:"keyless,omitempty"`

	// Attestor is a nested set of Attestor used to specify a more complex set of match authorities.
	// +kubebuilder:validation:Optional
	Attestor *apiextv1.JSON `json:"attestor,omitempty" yaml:"attestor,omitempty"`

	// Annotations are used for image verification.
	// Every specified key-value pair must exist and match in the verified payload.
	// The payload may contain other key-value pairs.
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`

	// Repository is an optional alternate OCI repository to use for signatures and attestations that match this rule.
	// If specified Repository will override other OCI image repository locations for this Attestor.
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
}

type StaticKeyAttestor struct {
	// Keys is a set of X.509 public keys used to verify image signatures. The keys can be directly
	// specified or can be a variable reference to a key specified in a ConfigMap (see
	// https://kyverno.io/docs/writing-policies/variables/), or reference a standard Kubernetes Secret
	// elsewhere in the cluster by specifying it in the format "k8s://<namespace>/<secret_name>".
	// The named Secret must specify a key `cosign.pub` containing the public key used for
	// verification, (see https://github.com/sigstore/cosign/blob/main/KMS.md#kubernetes-secret).
	// When multiple keys are specified each key is processed as a separate staticKey entry
	// (.attestors[*].entries.keys) within the set of attestors and the count is applied across the keys.
	PublicKeys string `json:"publicKeys,omitempty" yaml:"publicKeys,omitempty"`

	// Specify signature algorithm for public keys. Supported values are sha224, sha256, sha384 and sha512.
	// +kubebuilder:default=sha256
	SignatureAlgorithm string `json:"signatureAlgorithm,omitempty" yaml:"signatureAlgorithm,omitempty"`

	// KMS provides the URI to the public key stored in a Key Management System. See:
	// https://github.com/sigstore/cosign/blob/main/KMS.md
	KMS string `json:"kms,omitempty" yaml:"kms,omitempty"`

	// Reference to a Secret resource that contains a public key
	Secret *SecretReference `json:"secret,omitempty" yaml:"secret,omitempty"`

	// Rekor provides configuration for the Rekor transparency log service. If an empty object
	// is provided the public instance of Rekor (https://rekor.sigstore.dev) is used.
	// +kubebuilder:validation:Optional
	Rekor *Rekor `json:"rekor,omitempty" yaml:"rekor,omitempty"`

	// CTLog (certificate timestamp log) provides a configuration for validation of Signed Certificate
	// Timestamps (SCTs). If the value is unset, the default behavior by Cosign is used.
	// +kubebuilder:validation:Optional
	CTLog *CTLog `json:"ctlog,omitempty" yaml:"ctlog,omitempty"`
}

type SecretReference struct {
	// Name of the secret. The provided secret must contain a key named cosign.pub.
	Name string `json:"name" yaml:"name"`

	// Namespace name where the Secret exists.
	Namespace string `json:"namespace" yaml:"namespace"`
}

type CertificateAttestor struct {
	// Cert is an optional PEM-encoded public certificate.
	// +kubebuilder:validation:Optional
	Certificate string `json:"cert,omitempty" yaml:"cert,omitempty"`

	// CertChain is an optional PEM encoded set of certificates used to verify.
	// +kubebuilder:validation:Optional
	CertificateChain string `json:"certChain,omitempty" yaml:"certChain,omitempty"`

	// Rekor provides configuration for the Rekor transparency log service. If an empty object
	// is provided the public instance of Rekor (https://rekor.sigstore.dev) is used.
	// +kubebuilder:validation:Optional
	Rekor *Rekor `json:"rekor,omitempty" yaml:"rekor,omitempty"`

	// CTLog (certificate timestamp log) provides a configuration for validation of Signed Certificate
	// Timestamps (SCTs). If the value is unset, the default behavior by Cosign is used.
	// +kubebuilder:validation:Optional
	CTLog *CTLog `json:"ctlog,omitempty" yaml:"ctlog,omitempty"`
}

type KeylessAttestor struct {
	// Rekor provides configuration for the Rekor transparency log service. If an empty object
	// is provided the public instance of Rekor (https://rekor.sigstore.dev) is used.
	// +kubebuilder:validation:Optional
	Rekor *Rekor `json:"rekor,omitempty" yaml:"rekor,omitempty"`

	// CTLog (certificate timestamp log) provides a configuration for validation of Signed Certificate
	// Timestamps (SCTs). If the value is unset, the default behavior by Cosign is used.
	// +kubebuilder:validation:Optional
	CTLog *CTLog `json:"ctlog,omitempty" yaml:"ctlog,omitempty"`

	// Issuer is the certificate issuer used for keyless signing.
	// +kubebuilder:validation:Optional
	Issuer string `json:"issuer,omitempty" yaml:"issuer,omitempty"`

	// Subject is the verified identity used for keyless signing, for example the email address.
	// +kubebuilder:validation:Optional
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`

	// Roots is an optional set of PEM encoded trusted root certificates.
	// If not provided, the system roots are used.
	// +kubebuilder:validation:Optional
	Roots string `json:"roots,omitempty" yaml:"roots,omitempty"`

	// AdditionalExtensions are certificate-extensions used for keyless signing.
	// +kubebuilder:validation:Optional
	AdditionalExtensions map[string]string `json:"additionalExtensions,omitempty" yaml:"additionalExtensions,omitempty"`
}

type Rekor struct {
	// URL is the address of the transparency log. Defaults to the public Rekor log instance https://rekor.sigstore.dev.
	// +kubebuilder:validation:Required
	// +kubebuilder:Default:=https://rekor.sigstore.dev
	URL string `json:"url" yaml:"url"`

	// RekorPubKey is an optional PEM-encoded public key to use for a custom Rekor.
	// If set, this will be used to validate transparency log signatures from a custom Rekor.
	// +kubebuilder:validation:Optional
	RekorPubKey string `json:"pubkey,omitempty" yaml:"pubkey,omitempty"`

	// IgnoreTlog skips transparency log verification.
	// +kubebuilder:validation:Optional
	IgnoreTlog bool `json:"ignoreTlog,omitempty" yaml:"ignoreTlog,omitempty"`
}

type CTLog struct {
	// IgnoreSCT defines whether to use the Signed Certificate Timestamp (SCT) log to check for a certificate
	// timestamp. Default is false. Set to true if this was opted out during signing.
	// +kubebuilder:validation:Optional
	IgnoreSCT bool `json:"ignoreSCT,omitempty" yaml:"ignoreSCT,omitempty"`

	// PubKey, if set, is used to validate SCTs against a custom source.
	// +kubebuilder:validation:Optional
	CTLogPubKey string `json:"pubkey,omitempty" yaml:"pubkey,omitempty"`
}

// Attestation are checks for signed in-toto Statements that are used to verify the image.
// See https://github.com/in-toto/attestation. Kyverno fetches signed attestations from the
// OCI registry and decodes them into a list of Statements.
type Attestation struct {
	// Deprecated in favour of 'Type', to be removed soon
	// +kubebuilder:validation:Optional
	PredicateType string `json:"predicateType" yaml:"predicateType"`

	// Type defines the type of attestation contained within the Statement.
	// +kubebuilder:validation:Optional
	Type string `json:"type" yaml:"type"`

	// Attestors specify the required attestors (i.e. authorities).
	// +kubebuilder:validation:Optional
	Attestors []AttestorSet `json:"attestors" yaml:"attestors"`

	// Conditions are used to verify attributes within a Predicate. If no Conditions are specified
	// the attestation check is satisfied as long there are predicates that match the predicate type.
	// +kubebuilder:validation:Optional
	Conditions []AnyAllConditions `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

type ImageRegistryCredentials struct {
	// AllowInsecureRegistry allows insecure access to a registry.
	// +kubebuilder:validation:Optional
	AllowInsecureRegistry bool `json:"allowInsecureRegistry,omitempty" yaml:"allowInsecureRegistry,omitempty"`

	// Providers specifies a list of OCI Registry names, whose authentication providers are provided.
	// It can be of one of these values: default,google,azure,amazon,github.
	// +kubebuilder:validation:Optional
	Providers []ImageRegistryCredentialsProvidersType `json:"providers,omitempty" yaml:"providers,omitempty"`

	// Secrets specifies a list of secrets that are provided for credentials.
	// Secrets must live in the Kyverno namespace.
	// +kubebuilder:validation:Optional
	Secrets []string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

func (iv *ImageVerification) GetType() ImageVerificationType {
	if iv.Type != "" {
		return iv.Type
	}

	return Cosign
}

// Validate implements programmatic validation
func (iv *ImageVerification) Validate(isAuditFailureAction bool, path *field.Path) (errs field.ErrorList) {
	copy := iv.Convert()

	if isAuditFailureAction && iv.MutateDigest {
		errs = append(errs, field.Invalid(path.Child("mutateDigest"), iv.MutateDigest, "mutateDigest must be set to false for ‘Audit’ failure action"))
	}

	if len(copy.ImageReferences) == 0 {
		errs = append(errs, field.Invalid(path, iv, "An image reference is required"))
	}

	asPath := path.Child("attestations")
	for i, attestation := range copy.Attestations {
		attestationErrors := attestation.Validate(asPath.Index(i))
		errs = append(errs, attestationErrors...)
	}

	attestorsPath := path.Child("attestors")
	for i, as := range copy.Attestors {
		attestorErrors := as.Validate(attestorsPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	if iv.Type == Notary {
		for _, attestorSet := range iv.Attestors {
			for _, attestor := range attestorSet.Entries {
				if attestor.Keyless != nil {
					errs = append(errs, field.Invalid(attestorsPath, iv, "Keyless field is not allowed for type notary"))
				}
				if attestor.Keys != nil {
					errs = append(errs, field.Invalid(attestorsPath, iv, "Keys field is not allowed for type notary"))
				}
			}
		}
	}

	return errs
}

func (a *Attestation) Validate(path *field.Path) (errs field.ErrorList) {
	if len(a.Attestors) == 0 {
		return
	}

	attestorsPath := path.Child("attestors")
	for i, as := range a.Attestors {
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
	}

	entriesPath := path.Child("entries")
	for i, e := range as.Entries {
		attestorErrors := e.Validate(entriesPath.Index(i))
		errs = append(errs, attestorErrors...)
	}

	return errs
}

func (a *Attestor) Validate(path *field.Path) (errs field.ErrorList) {
	if (a.Keys != nil && (a.Certificates != nil || a.Keyless != nil || a.Attestor != nil)) ||
		(a.Certificates != nil && (a.Keys != nil || a.Keyless != nil || a.Attestor != nil)) ||
		(a.Keyless != nil && (a.Certificates != nil || a.Keys != nil || a.Attestor != nil)) ||
		(a.Attestor != nil && (a.Certificates != nil || a.Keys != nil || a.Keyless != nil)) ||
		(a.Keys == nil && a.Certificates == nil && a.Keyless == nil && a.Attestor == nil) {
		errs = append(errs, field.Invalid(path, a, "keys, certificates, keyless, or a nested attestor is required"))
	}

	if a.Keys != nil {
		staticKeyPath := path.Child("keys")
		staticKeyErrors := a.Keys.Validate(staticKeyPath)
		errs = append(errs, staticKeyErrors...)
	}

	if a.Certificates != nil {
		certificatesPath := path.Child("certificates")
		certificatesErrors := a.Certificates.Validate(certificatesPath)
		errs = append(errs, certificatesErrors...)
	}

	if a.Keyless != nil {
		keylessPath := path.Child("keyless")
		keylessErrors := a.Keyless.Validate(keylessPath)
		errs = append(errs, keylessErrors...)
	}

	if a.Attestor != nil {
		attestorPath := path.Child("attestor")
		attestorSet, err := AttestorSetUnmarshal(a.Attestor)
		if err != nil {
			fieldErr := field.Invalid(attestorPath, a.Attestor, err.Error())
			errs = append(errs, fieldErr)
		} else {
			attestorErrors := validateAttestorSet(attestorSet, attestorPath)
			errs = append(errs, attestorErrors...)
		}
	}

	return errs
}

func AttestorSetUnmarshal(o *apiextv1.JSON) (*AttestorSet, error) {
	var as AttestorSet
	if err := json.Unmarshal(o.Raw, &as); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestor set %s: %w", string(o.Raw), err)
	}

	return &as, nil
}

func (ska *StaticKeyAttestor) Validate(path *field.Path) (errs field.ErrorList) {
	if ska.PublicKeys == "" && ska.KMS == "" && ska.Secret == nil {
		errs = append(errs, field.Invalid(path, ska, "A public key, kms key or secret is required"))
	}
	if ska.PublicKeys != "" {
		if _, ok := signatureAlgorithmMap[ska.SignatureAlgorithm]; !ok {
			errs = append(errs, field.Invalid(path, ska, "Invalid signature algorithm provided"))
		}
	}
	return errs
}

func (ca *CertificateAttestor) Validate(path *field.Path) (errs field.ErrorList) {
	if ca.Certificate == "" && ca.CertificateChain == "" {
		errs = append(errs, field.Invalid(path, ca, "cert or certChain required"))
	}

	return errs
}

func (ka *KeylessAttestor) Validate(path *field.Path) (errs field.ErrorList) {
	if ka.Rekor == nil && ka.Roots == "" {
		errs = append(errs, field.Invalid(path, ka, "Either Rekor URL or roots are required"))
	}

	if ka.Rekor != nil && ka.Rekor.URL == "" {
		errs = append(errs, field.Invalid(path, ka, "An URL is required"))
	}

	return errs
}

func (iv *ImageVerification) Convert() *ImageVerification {
	if iv.Image == "" && iv.Key == "" && iv.Issuer == "" {
		return iv
	}

	copy := iv.DeepCopy()
	copy.Image = ""
	copy.Issuer = ""
	copy.Subject = ""
	copy.Roots = ""

	if iv.Image != "" {
		copy.ImageReferences = append(copy.ImageReferences, iv.Image)
	}

	attestorSet := AttestorSet{}
	if len(iv.Annotations) > 0 || iv.Key != "" || iv.Issuer != "" {
		attestor := Attestor{
			Annotations: iv.Annotations,
		}

		if iv.Key != "" {
			attestor.Keys = &StaticKeyAttestor{
				PublicKeys: iv.Key,
			}
		} else if iv.Issuer != "" {
			attestor.Keyless = &KeylessAttestor{
				Issuer:  iv.Issuer,
				Subject: iv.Subject,
				Roots:   iv.Roots,
			}
		}

		attestorSet.Entries = append(attestorSet.Entries, attestor)
		if len(iv.Attestations) > 0 {
			for i := range iv.Attestations {
				copy.Attestations[i].Attestors = append(copy.Attestations[i].Attestors, attestorSet)
			}
		} else {
			copy.Attestors = append(copy.Attestors, attestorSet)
		}
	}

	copy.Attestations = iv.Attestations
	return copy
}
