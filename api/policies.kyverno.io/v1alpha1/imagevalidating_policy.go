package v1alpha1

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=imagevalidatingpolicies,scope="Cluster",shortName=ivpol,categories=kyverno
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditionStatus.ready`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageValidatingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ImageValidatingPolicySpec `json:"spec"`
	// Status contains policy runtime data.
	// +optional
	Status ImageValidatingPolicyStatus `json:"status,omitempty"`
}

// BackgroundEnabled checks if background is set to true
func (s ImageValidatingPolicy) BackgroundEnabled() bool {
	return s.Spec.BackgroundEnabled()
}

type ImageValidatingPolicyStatus struct {
	// +optional
	ConditionStatus ConditionStatus `json:"conditionStatus,omitempty"`

	// +optional
	Autogen ImageValidatingPolicyAutogenStatus `json:"autogen,omitempty"`
}

func (s *ImageValidatingPolicy) GetMatchConstraints() admissionregistrationv1.MatchResources {
	if s.Spec.MatchConstraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *s.Spec.MatchConstraints
}

func (s *ImageValidatingPolicy) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	return s.Spec.MatchConditions
}

func (s *ImageValidatingPolicy) GetWebhookConfiguration() *WebhookConfiguration {
	return s.Spec.WebhookConfiguration
}

func (s *ImageValidatingPolicy) GetFailurePolicy() admissionregistrationv1.FailurePolicyType {
	if toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore() {
		return admissionregistrationv1.Ignore
	}
	if s.Spec.FailurePolicy == nil {
		return admissionregistrationv1.Fail
	}
	return *s.Spec.FailurePolicy
}

func (s *ImageValidatingPolicy) GetVariables() []admissionregistrationv1.Variable {
	return s.Spec.Variables
}

func (s *ImageValidatingPolicy) GetSpec() *ImageValidatingPolicySpec {
	return &s.Spec
}

func (s *ImageValidatingPolicy) GetStatus() *ImageValidatingPolicyStatus {
	return &s.Status
}

func (s *ImageValidatingPolicy) GetKind() string {
	return "ImageValidatingPolicy"
}

// AdmissionEnabled checks if admission is set to true
func (s ImageValidatingPolicySpec) AdmissionEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

// BackgroundEnabled checks if background is set to true
func (s ImageValidatingPolicySpec) BackgroundEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Background == nil || s.EvaluationConfiguration.Background.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Background.Enabled
}

// ValidationActions returns the validation actions.
func (s ImageValidatingPolicySpec) ValidationActions() []admissionregistrationv1.ValidationAction {
	const defaultValue = admissionregistrationv1.Deny
	if len(s.ValidationAction) == 0 {
		return []admissionregistrationv1.ValidationAction{defaultValue}
	}
	return s.ValidationAction
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ImageValidatingPolicyList is a list of ImageValidatingPolicy instances
type ImageValidatingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ImageValidatingPolicy `json:"items"`
}

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

// ImageValidatingPolicySpec is the specification of the desired behavior of the ImageValidatingPolicy.
type ImageValidatingPolicySpec struct {
	// MatchConstraints specifies what resources this policy is designed to validate.
	// +optional
	MatchConstraints *admissionregistrationv1.MatchResources `json:"matchConstraints"`

	// FailurePolicy defines how to handle failures for the admission policy. Failures can
	// occur from CEL expression parse errors, type check errors, runtime errors and invalid
	// or mis-configured policy definitions or bindings.
	// +optional
	// +kubebuilder:validation:Enum=Ignore;Fail
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy"`

	// auditAnnotations contains CEL expressions which are used to produce audit
	// annotations for the audit event of the API request.
	// validations and auditAnnotations may not both be empty; a least one of validations or auditAnnotations is
	// required.
	// +listType=atomic
	// +optional
	AuditAnnotations []admissionregistrationv1.AuditAnnotation `json:"auditAnnotations,omitempty"`

	// ValidationAction specifies the action to be taken when the matched resource violates the policy.
	// Required.
	// +listType=set
	// +kubebuilder:validation:items:Enum=Deny;Audit;Warn
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

	// ValidationConfigurations defines settings for mutating and verifying image digests, and enforcing image verification through signatures.
	// +optional
	// +kubebuilder:default={}
	ValidationConfigurations ValidationConfiguration `json:"validationConfigurations"`

	// MatchImageReferences is a list of Glob and CELExpressions to match images.
	// Any image that matches one of the rules is considered for validation
	// Any image that does not match a rule is skipped, even when they are passed as arguments to
	// image verification functions
	// +optional
	MatchImageReferences []MatchImageReference `json:"matchImageReferences"`

	// Credentials provides credentials that will be used for authentication with registry.
	// +kubebuilder:validation:Optional
	Credentials *Credentials `json:"credentials,omitempty"`

	// ImageExtractors is a list of CEL expression to extract images from the resource
	// +optional
	ImageExtractors []ImageExtractor `json:"images,omitempty"`

	// Attestors provides a list of trusted authorities.
	Attestors []Attestor `json:"attestors"`

	// Attestations provides a list of image metadata to verify
	// +optional
	Attestations []Attestation `json:"attestations"`

	// Validations contain CEL expressions which is used to apply the image validation checks.
	// +listType=atomic
	Validations []admissionregistrationv1.Validation `json:"validations"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// EvaluationConfiguration defines the configuration for the policy evaluation.
	// +optional
	EvaluationConfiguration *EvaluationConfiguration `json:"evaluation,omitempty"`

	// AutogenConfiguration defines the configuration for the generation controller.
	// +optional
	AutogenConfiguration *ImageValidatingPolicyAutogenConfiguration `json:"autogen,omitempty"`
}

// MatchImageReference defines a Glob or a CEL expression for matching images
// +kubebuilder:oneOf:={required:{glob}}
// +kubebuilder:oneOf:={required:{expression}}
type MatchImageReference struct {
	// Glob defines a globbing pattern for matching images
	// +optional
	Glob string `json:"glob,omitempty"`
	// Expression defines CEL Expressions for matching images
	// +optional
	Expression string `json:"expression,omitempty"`
}

type ValidationConfiguration struct {
	// MutateDigest enables replacement of image tags with digests.
	// Defaults to true.
	// +kubebuilder:default=true
	// +optional
	MutateDigest *bool `json:"mutateDigest,omitempty"`

	// VerifyDigest validates that images have a digest.
	// +kubebuilder:default=true
	// +optional
	VerifyDigest *bool `json:"verifyDigest,omitempty"`

	// Required validates that images are verified, i.e., have passed a signature or attestation check.
	// +kubebuilder:default=true
	// +optional
	Required *bool `json:"required,omitempty"`
}

type ImageExtractor struct {
	// Name is the name for this imageList. It is used to refer to the images in verification block as images.<name>
	Name string `json:"name"`

	// Expression defines CEL expression to extract images from the resource.
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
// +kubebuilder:oneOf:={required:{cosign}}
// +kubebuilder:oneOf:={required:{notary}}
type Attestor struct {
	// Name is the name for this attestor. It is used to refer to the attestor in verification
	Name string `json:"name"`
	// Cosign defines attestor configuration for Cosign based signatures
	// +optional
	Cosign *Cosign `json:"cosign,omitempty"`
	// Notary defines attestor configuration for Notary based signatures
	// +optional
	Notary *Notary `json:"notary,omitempty"`
}

func (v Attestor) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if reflect.TypeOf(v).AssignableTo(typeDesc) {
		return v, nil
	}
	return nil, fmt.Errorf("type conversion error from 'Image' to '%v'", typeDesc)
}

func (v Attestor) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case cel.ObjectType("imageverify.attestor"):
		return v
	default:
		return types.NewErr("type conversion error from '%s' to '%s'", cel.ObjectType("imageverify.attestor"), typeVal)
	}
}

func (v Attestor) Equal(other ref.Val) ref.Val {
	img, ok := other.(Attestor)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}
	return types.Bool(reflect.DeepEqual(v, img))
}

func (v Attestor) Type() ref.Type {
	return cel.ObjectType("imageverify.attestor")
}

func (v Attestor) Value() any {
	return v
}

func (a Attestor) GetKey() string {
	return a.Name
}

func (a Attestor) IsCosign() bool {
	return a.Cosign != nil
}

func (a Attestor) IsNotary() bool {
	return a.Notary != nil
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
	Source *Source `json:"source,omitempty"`
	// CTLog sets the configuration to verify the authority against a Rekor instance.
	// +optional
	CTLog *CTLog `json:"ctlog,omitempty"`
	// TUF defines the configuration to fetch sigstore root
	// +optional
	TUF *TUF `json:"tuf,omitempty"`
	// Annotations are used for image verification.
	// Every specified key-value pair must exist and match in the verified payload.
	// The payload may contain other key-value pairs.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// StringOrExpression contains either a raw string input or a CEL expression
// +kubebuilder:oneOf:={required:{value}}
// +kubebuilder:oneOf:={required:{expression}}
type StringOrExpression struct {
	// Value defines the raw string input.
	// +optional
	Value string `json:"value,omitempty"`
	// Expression defines the a CEL expression input.
	// +optional
	Expression string `json:"expression,omitempty"`
}

// Notary defines attestor configuration for Notary based signatures
type Notary struct {
	// Certs define the cert chain for Notary signature verification
	// +optional
	Certs *StringOrExpression `json:"certs,omitempty"`
	// TSACerts define the cert chain for verifying timestamps of notary signature
	// +optional
	TSACerts *StringOrExpression `json:"tsaCerts,omitempty"`
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
	TagPrefix string `json:"tagPrefix,omitempty"`
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
	// TSACertChain, if set, is the PEM-encoded certificate chain file for the RFC3161 timestamp authority. Must
	// contain the root CA certificate. Optionally may contain intermediate CA certificates, and
	// may contain the leaf TSA certificate if not present in the timestamurce.
	// +optional
	TSACertChain string `json:"tsaCertChain,omitempty"`
	// InsecureIgnoreTlog skips transparency log verification.
	// +optional
	InsecureIgnoreTlog bool `json:"insecureIgnoreTlog,omitempty"`
	// IgnoreSCT defines whether to use the Signed Certificate Timestamp (SCT) log to check for a certificate
	// timestamp. Default is false. Set to true if this was opted out during signing.
	// +optional
	InsecureIgnoreSCT bool `json:"insecureIgnoreSCT,omitempty"`
}

// A Key must specify only one of CEL, Data or KMS
type Key struct {
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
	// Expression is a Expression expression that returns the public key.
	// +optional
	Expression string `json:"expression,omitempty"`
}

// Keyless contains location of the validating certificate and the identities
// against which to verify.
type Keyless struct {
	// Identities sets a list of identities.
	Identities []Identity `json:"identities"`
	// Roots is an optional set of PEM encoded trusted root certificates.
	// If not provided, the system roots are used.
	// +kubebuilder:validation:Optional
	Roots string `json:"roots,omitempty"`
}

// Certificate defines the configuration for local signature verification
type Certificate struct {
	// Certificate is the to the public certificate for local signature verification.
	// +optional
	Certificate *StringOrExpression `json:"cert,omitempty"`
	// CertificateChain is the list of CA certificates in PEM format which will be needed
	// when building the certificate chain for the signing certificate. Must start with the
	// parent intermediate CA certificate of the signing certificate and end with the root certificate
	// +optional
	CertificateChain *StringOrExpression `json:"certChain,omitempty"`
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
	InToto *InToto `json:"intoto,omitempty"`

	// Referrer defines the details of attestation attached using OCI 1.1 format
	// +optional
	Referrer *Referrer `json:"referrer,omitempty"`
}

func (a Attestation) GetKey() string {
	return a.Name
}

func (a Attestation) IsInToto() bool {
	return a.InToto != nil
}

func (a Attestation) IsReferrer() bool {
	return a.Referrer != nil
}

type InToto struct {
	// Type defines the type of attestation contained within the statement.
	Type string `json:"type"`
}

type Referrer struct {
	// Type defines the type of attestation attached to the image.
	Type string `json:"type"`
}

// EvaluationMode returns the evaluation mode of the policy.
func (s ImageValidatingPolicySpec) EvaluationMode() EvaluationMode {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Mode == "" {
		return EvaluationModeKubernetes
	}
	return s.EvaluationConfiguration.Mode
}

type ImageValidatingPolicyAutogenConfiguration struct {
	// PodControllers specifies whether to generate a pod controllers rules.
	PodControllers *PodControllersGenerationConfiguration `json:"podControllers,omitempty"`
}
