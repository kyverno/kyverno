package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
)

// CredentialsProvidersType provides the list of credential providers required.
// +kubebuilder:validation:Enum=default;amazon;azure;google;github
type CredentialsProvidersType string

const (
	DEFAULT CredentialsProvidersType = "default"
	AWS     CredentialsProvidersType = "amazon"
	ACR     CredentialsProvidersType = "azure"
	GCP     CredentialsProvidersType = "google"
	GHCR    CredentialsProvidersType = "github"
)

// ImageVerificationPolicySpec is the specification of the desired behavior of the ImageVerificationPolicy.
type ImageVerificationPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to validate.
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints"`

	// FailurePolicy defines how to handle failures for the admission policy. Failures can
	// occur from CEL expression parse errors, type check errors, runtime errors and invalid
	// or mis-configured policy definitions or bindings.
	// +optional
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy"`

	// ValidationAction specifies the action to be taken when the matched resource violates the policy.
	// Required.
	// +listType=set
	ValidationAction []admissionregistrationv1.ValidationAction `json:"validationActions,omitempty"`

	// MatchConditions is a list of conditions that must be met for a request to be validated.
	// Match conditions filter requests that have already been matched by the rules,
	// namespaceSelector, and objectSelector. An empty list of matchConditions matches all requests.
	// There are a maximum of 64 match conditions allowed.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`

	// Variables contain definitions of variables that can be used in composition of other expressions.
	// Each variable is defined as a named CEL expression.
	// +optional
	Variables []admissionregistrationv1.Variable `json:"variables,omitempty"`

	// ImagesRules is a list of Glob and CELExpressions to match images.
	// Any image that matches one of the rules is considered for validation
	// Any image that does not match a rule is skipped, even when they are passed as arguments to
	// image verification functions
	// +optional
	ImageRules []ImageRule `json:"imageRules"`

	// MutateDigest enables replacement of image tags with digests.
	// Defaults to true.
	// +kubebuilder:default=true
	// +optional
	MutateDigest *bool `json:"mutateDigest"`

	// VerifyDigest validates that images have a digest.
	// +kubebuilder:default=true
	// +optional
	VerifyDigest *bool `json:"verifyDigest"`

	// Required validates that images are verified i.e. have matched passed a signature or attestation check.
	// +kubebuilder:default=true
	// +optional
	Required *bool `json:"required"`

	// Credentials provides credentials that will be used for authentication with registry.
	// +kubebuilder:validation:Optional
	Credentials *Credentials `json:"credentials,omitempty"`

	// Images is a list of CEL expression to extract images from the resource
	// +optional
	Images []Image `json:"images,omitempty"`

	// Attestors provides a list of trusted authorities.
	Attestors []Attestor `json:"attestors"`

	// Attestations provides a list of image metadata to verify
	// +optional
	Attestations []Attestation `json:"attestations"`

	// Verifications contain CEL expressions which is used to apply the image verification checks.
	// +listType=atomic
	Verifications []admissionregistrationv1.Validation `json:"verifications"`
}

// ImageRule defines a Glob or a CEL expression for matching images
type ImageRule struct {
	// Glob defines a globbing pattern for matching images
	// +optional
	Glob string `json:"glob"`
	// Cel defines CEL Expressions for matching images
	CELExpression string `json:"cel"`
}

type Image struct {
	// Name is the name for this imageList. It is used to refer to the images in verification block as images.<name>
	Name string `json:"name"`

	// Expression defines CEL expression to extact images from the resource.
	Expression string `json:"expression"`
}

type Credentials struct {
	// AllowInsecureRegistry allows insecure access to a registry.
	// +optional
	AllowInsecureRegistry bool `json:"allowInsecureRegistry,omitempty"`

	// Providers specifies a list of OCI Registry names, whose authentication providers are provided.
	// It can be of one of these values: default,google,azure,amazon,github.
	// +optional
	Providers []CredentialsProvidersType `json:"providers,omitempty"`

	// Secrets specifies a list of secrets that are provided for credentials.
	// Secrets must live in the Kyverno namespace.
	// +optional
	Secrets []string `json:"secrets,omitempty"`
}

// Attestor is an identity that confirms or verifies the authenticity of an image or an attestation
type Attestor struct {
	// Name is the name for this attestor. It is used to refer to the attestor in verification
	Name string `json:"name"`
	// Cosign defines attestor configuration for Cosign based signatures
	// +optional
	Cosign Cosign `json:"cosign,omitempty"`
	// Notary defines attestor configuration for Notary based signatures
	// +optional
	Notary Notary `json:"notary,omitempty"`
}

// Cosign defines attestor configuration for Cosign based signatures
type Cosign struct {
	// Key defines the type of key to validate the image.
	// +optional
	Key *Key `json:"key,omitempty"`
	// Keyless sets the configuration to verify the authority against a Fulcio instance.
	// +optional
	Keyless *Keyless `json:"keyless,omitempty"`
	// Certificate defines the configuration for local signature verification
	// +optional
	Certificate *Certificate `json:"certificate,omitempty"`
	// Sources sets the configuration to specify the sources from where to consume the signature and attestations.
	// +optional
	Sources []Source `json:"source,omitempty"`
	// CTLog sets the configuration to verify the authority against a Rekor instance.
	// +optional
	CTLog *CTLog `json:"ctlog,omitempty"`
	// TUF defines the configuration to fetch sigstore root
	// +optional
	TUF *TUF `json:"tuf,omitempty"`
}

// Notary defines attestor configuration for Notary based signatures
type Notary struct {
	// Certs define the cert chain for Notary signature verification
	Certs string `json:"certs"`
	// TSACerts define the cert chain for verifying timestamps of notary signature
	// +optional
	TSACerts string `json:"tsaCerts"`
}

// TUF defines the configuration to fetch sigstore root
type TUF struct {
	// Root defines the path or data of the trusted root
	// +optional
	Root TUFRoot `json:"root,omitempty"`
	// Mirror is the base URL of Sigstore TUF repository
	// +optional
	Mirror string `json:"mirror,omitempty"`
}

// TUFRoot defines the path or data of the trusted root
type TUFRoot struct {
	// Path is the URL or File location of the TUF root
	// +optional
	Path string `json:"path,omitempty"`
	// Data is the base64 encoded TUF root
	// +optional
	Data string `json:"data,omitempty"`
}

// Source specifies the location of the signature / attestations.
type Source struct {
	// Repository defines the location from where to pull the signature / attestations.
	// +optional
	Repository string `json:"repository,omitempty"`
	// SignaturePullSecrets is an optional list of references to secrets in the
	// same namespace as the deploying resource for pulling any of the signatures
	// used by this Source.
	// +optional
	SignaturePullSecrets []corev1.LocalObjectReference `json:"PullSecrets,omitempty"`
	// TagPrefix is an optional prefix that signature and attestations have.
	// This is the 'tag based discovery' and in the future once references are
	// fully supported that should likely be the preferred way to handle these.
	// +optional
	TagPrefix *string `json:"tagPrefix,omitempty"`
}

// CTLog sets the configuration to verify the authority against a Rekor instance.
type CTLog struct {
	// URL sets the url to the rekor instance (by default the public rekor.sigstore.dev)
	// +optional
	URL string `json:"url,omitempty"`
	// RekorPubKey is an optional PEM-encoded public key to use for a custom Rekor.
	// If set, this will be used to validate transparency log signatures from a custom Rekor.
	// +optional
	RekorPubKey string `json:"rekorPubKey,omitempty"`
	// CTLogPubKey, if set, is used to validate SCTs against a custom source.
	// +optional
	CTLogPubKey string `json:"ctLogPubKey,omitempty"`
	// InsecureIgnoreTlog skips transparency log verification.
	// +optional
	InsecureIgnoreTlog bool `json:"insecureIgnoreTlog,omitempty"`
	// IgnoreSCT defines whether to use the Signed Certificate Timestamp (SCT) log to check for a certificate
	// timestamp. Default is false. Set to true if this was opted out during signing.
	// +optional
	InsecureIgnoreSCT bool `json:"insecureIgnoreSCT,omitempty"`
}

// This references a public verification key stored in
// a secret in the kyverno namespace.
// A Key must specify only one of SecretRef, Data or KMS
type Key struct {
	// SecretRef sets a reference to a secret with the key.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
	// Data contains the inline public key
	// +optional
	Data string `json:"data,omitempty"`
	// KMS contains the KMS url of the public key
	// Supported formats differ based on the KMS system used.
	// +optional
	KMS string `json:"kms,omitempty"`
	// HashAlgorithm specifues signature algorithm for public keys. Supported values are
	// sha224, sha256, sha384 and sha512. Defaults to sha256.
	// +optional
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
}

// Keyless contains location of the validating certificate and the identities
// against which to verify. KeylessRef will contain either the URL to the verifying
// certificate, or it will contain the certificate data inline or in a secret.
type Keyless struct {
	// URL defines a url to the keyless instance.
	// +optional
	URL string `json:"url,omitempty"`
	// Identities sets a list of identities.
	Identities []Identity `json:"identities"`
	// CACert sets a reference to CA certificate
	// +optional
	CACert *Key `json:"ca-cert,omitempty"`
}

// Certificate defines the configuration for local signature verification
type Certificate struct {
	// Certificate is the to the public certificate for local signature verification.
	// +optional
	Certificate string `json:"cert,omitempty"`
	// CertificateChain is the list of CA certificates in PEM format which will be needed
	// when building the certificate chain for the signing certificate. Must start with the
	// parent intermediate CA certificate of the signing certificate and end with the root certificate
	// +optional
	CertificateChain string `json:"certChain,omitempty"`
}

// Identity may contain the issuer and/or the subject found in the transparency
// log.
// Issuer/Subject uses a strict match, while IssuerRegExp and SubjectRegExp
// apply a regexp for matching.
type Identity struct {
	// Issuer defines the issuer for this identity.
	// +optional
	Issuer string `json:"issuer,omitempty"`
	// Subject defines the subject for this identity.
	// +optional
	Subject string `json:"subject,omitempty"`
	// IssuerRegExp specifies a regular expression to match the issuer for this identity.
	// +optional
	IssuerRegExp string `json:"issuerRegExp,omitempty"`
	// SubjectRegExp specifies a regular expression to match the subject for this identity.
	// +optional
	SubjectRegExp string `json:"subjectRegExp,omitempty"`
}

// Attestation defines the identification details of the  metadata that has to be verified
type Attestation struct {
	// Name is the name for this attestation. It is used to refer to the attestation in verification
	Name string `json:"name"`

	// InToto defines the details of attestation attached using intoto format
	// +optional
	InToto InToto `json:"intoto,omitempty"`

	// Referrer defines the details of attestation attached using OCI 1.1 format
	// +optional
	Referrer Referrer `json:"referrer,omitempty"`
}

type InToto struct {
	// Type defines the type of attestation contained within the statement.
	Type string `json:"type"`
}

type Referrer struct {
	// Type defines the type of attestation attached to the image.
	Type string `json:"type"`
}
