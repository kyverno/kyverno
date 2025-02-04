package v2alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// Attestor is an identity that confirms or verifies the authenticity of an image or an attestation
type Attestor struct {
	// Name is the name for this attestor. It is used to refer to the attestor in verification
	Name string `json:"name"`
	// Cosign defines attestor configuration for Cosign based signatures
	// +optional
	Cosign Cosign `json:"cosign"`
	// Notary defines attestor configuration for Notary based signatures
	// +optional
	Notary Notary `json:"notary"`
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
}

// TUF defines the configuration to fetch sigstore root
type TUF struct {
	// Root defines the path or data of the trusted root
	// +optional
	Root TUFRoot `json:"root"`
	// Mirror is the base URL of Sigstore TUF repository
	// +optional
	Mirror string `json:"mirror"`
}

// TUFRoot defines the path or data of the trusted root
type TUFRoot struct {
	// Path is the URL or File location of the TUF root
	// +optional
	Path string `json:"path"`
	// Data is the base64 encoded TUF root
	// +optional
	Data string `json:"ata"`
}

// Source specifies the location of the signature / attestations.
type Source struct {
	// Repository defines the location from where to pull the signature / attestations.
	// +optional
	Repository string `json:"repository"`
	// SignaturePullSecrets is an optional list of references to secrets in the
	// same namespace as the deploying resource for pulling any of the signatures
	// used by this Source.
	// +optional
	SignaturePullSecrets []corev1.LocalObjectReference `json:"PullSecrets,omitempty"`
	// TagPrefix is an optional prefix that signature and attestations have.
	// This is the 'tag based discovery' and in the future once references are
	// fully supported that should likely be the preferred way to handle these.
	// +optional
	TagPrefix *string `json:"tagPrefixy"`
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
// a secret in the cosign-system namespace.
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
