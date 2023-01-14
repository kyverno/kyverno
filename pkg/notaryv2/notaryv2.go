package notaryv2

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/notaryproject/notation-core-go/signature"
	"github.com/notaryproject/notation-go"
	notationregistry "github.com/notaryproject/notation-go/registry"
	sig "github.com/notaryproject/notation-go/signature"
	"github.com/notaryproject/notation-go/verification"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"go.uber.org/multierr"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"

	_ "github.com/notaryproject/notation-core-go/signature/cose"
	_ "github.com/notaryproject/notation-core-go/signature/jws"
)

func NewVerifier() images.ImageVerifier {
	return &notaryV2Verifier{
		log: logging.WithName("NotaryV2"),
	}
}

type notaryV2Verifier struct {
	trustCerts            []*x509.Certificate
	trustedX509Identities map[string]string
	log                   logr.Logger
}

func (v *notaryV2Verifier) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	v.log.V(2).Info("verifying image", "reference", opts.ImageRef)

	var err error
	certs := combineCerts(opts)
	v.trustCerts, err = cryptoutils.LoadCertificatesFromPEM(bytes.NewReader([]byte(certs)))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse certificates")
	}

	if opts.Identities != "" {
		v.trustedX509Identities, err = parseDistinguishedName(opts.Identities)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse identities")
		}
	}

	repo, parsedRef, err := parseReference(ctx, opts.ImageRef, opts.RegistryClient)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference: %s", opts.ImageRef)
	}

	// check that a digest is received
	artifactDigest, err := parsedRef.Digest()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to extract digest from %s", parsedRef.String())
	}

	artifactDescriptor, err := repo.Resolve(context.Background(), artifactDigest.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve artifact descriptor for %s", artifactDigest.String())
	}

	// get signature manifests
	sigManifests, err := repo.ListSignatureManifests(context.Background(), artifactDescriptor.Digest)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve digital signature(s) associated with %q from the registry", parsedRef.String())
	}

	if len(sigManifests) < 1 {
		return nil, errors.Errorf("no signatures are associated with %q, make sure the image was signed successfully", parsedRef.String())
	}

	v.log.V(2).Info("processing signature", "count", len(sigManifests))

	// process signatures
	var verificationOutcomes []*verification.SignatureVerificationOutcome
	for _, sigManifest := range sigManifests {
		// get signature envelope
		sigBlob, err := repo.GetBlob(context.Background(), sigManifest.Blob.Digest)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to retrieve digital signature with digest %q associated with %q from the registry", sigManifest.Blob.Digest, parsedRef.String())
		}

		outcome := &verification.SignatureVerificationOutcome{
			VerificationResults: []*verification.VerificationResult{},
			VerificationLevel:   verification.Strict,
		}

		err = v.processSignature(context.Background(), sigBlob, sigManifest, outcome)
		if err != nil {
			outcome.Error = err
		}

		verificationOutcomes = append(verificationOutcomes, outcome)
	}

	if len(verificationOutcomes) == 0 {
		return nil, errors.Errorf("missing verification outcomes")
	}

	// check whether verification was successful or not
	var errs []error
	for _, outcome := range verificationOutcomes {
		if outcome.Error != nil {
			errs = append(errs, outcome.Error)
			continue
		}

		// artifact digest must match the digest from the signature payload
		payload := &notation.Payload{}
		err := json.Unmarshal(outcome.EnvelopeContent.Payload.Content, payload)
		if err != nil || !artifactDescriptor.Equal(payload.TargetArtifact) {
			outcome.Error = fmt.Errorf("given digest %q does not match the digest %q present in the digital signature", artifactDigest, payload.TargetArtifact.Digest.String())
			continue
		}

		// TODO verify annotations
		outcome.SignedAnnotations = payload.TargetArtifact.Annotations

		// signature verification succeeds if there is at least one good signature
		return &images.Response{Digest: artifactDigest.String()}, nil
	}

	return nil, multierr.Combine(errs...)
}

func combineCerts(opts images.Options) string {
	certs := opts.Cert
	if opts.CertChain != "" {
		if certs != "" {
			certs = certs + "\n"
		}

		certs = certs + opts.CertChain
	}

	return certs
}

func parseReference(ctx context.Context, ref string, registryClient registryclient.Client) (*notationregistry.RepositoryClient, registry.Reference, error) {
	parsedRef, err := registry.ParseReference(ref)
	if err != nil {
		return nil, registry.Reference{}, errors.Wrapf(err, "failed to parse registry reference %s", ref)
	}

	authClient, plainHTTP, err := getAuthClient(ctx, parsedRef, registryClient)
	if err != nil {
		return nil, registry.Reference{}, err
	}

	repository := notationregistry.NewRepositoryClient(authClient, parsedRef, plainHTTP)
	parsedRef, err = resolveDigest(repository, parsedRef)
	if err != nil {
		return nil, registry.Reference{}, errors.Wrapf(err, "failed to resolve digest")
	}

	return repository, parsedRef, nil
}

type imageResource struct {
	ref registry.Reference
}

func (ir *imageResource) String() string {
	return ir.ref.String()
}

func (ir *imageResource) RegistryStr() string {
	return ir.ref.Registry
}

func getAuthClient(ctx context.Context, ref registry.Reference, rc registryclient.Client) (*auth.Client, bool, error) {
	if err := rc.RefreshKeychainPullSecrets(ctx); err != nil {
		return nil, false, errors.Wrapf(err, "failed to refresh image pull secrets")
	}

	authn, err := rc.Keychain().Resolve(&imageResource{ref})
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to resolve auth for %s", ref.String())
	}

	authConfig, err := authn.Authorization()
	if err != nil {
		return nil, false, errors.Wrapf(err, "failed to get auth config for %s", ref.String())
	}

	// if authConfig.Username == "" || authConfig.Password == "" {
	// 	return nil, false, errors.Errorf("failed to get registry credentials")
	// }

	credentials := auth.Credential{
		Username:     authConfig.Username,
		Password:     authConfig.Password,
		AccessToken:  authConfig.IdentityToken,
		RefreshToken: authConfig.RegistryToken,
	}

	authClient := &auth.Client{
		Credential: func(ctx context.Context, registry string) (auth.Credential, error) {
			switch registry {
			default:
				return credentials, nil
			}
		},
		Cache:    auth.NewCache(),
		ClientID: "notation",
	}

	authClient.SetUserAgent("notation/test")
	return authClient, false, nil
}

func resolveDigest(repo *notationregistry.RepositoryClient, ref registry.Reference) (registry.Reference, error) {
	if isDigestReference(ref.String()) {
		return ref, nil
	}

	// Resolve tag reference to digest reference.
	manifestDesc, err := getManifestDescriptorFromReference(repo, ref.String())
	if err != nil {
		return registry.Reference{}, err
	}

	ref.Reference = manifestDesc.Digest.String()
	return ref, nil
}

func isDigestReference(reference string) bool {
	parts := strings.SplitN(reference, "/", 2)
	if len(parts) == 1 {
		return false
	}

	index := strings.Index(parts[1], "@")
	return index != -1
}

func getManifestDescriptorFromReference(repo *notationregistry.RepositoryClient, reference string) (notation.Descriptor, error) {
	ref, err := registry.ParseReference(reference)
	if err != nil {
		return notation.Descriptor{}, err
	}

	return repo.Resolve(context.Background(), ref.ReferenceOrDefault())
}

// isCriticalFailure checks whether a VerificationResult fails the entire signature verification workflow.
// signature verification workflow is considered failed if there is a VerificationResult with "Enforced" as the action but the result was unsuccessful
func isCriticalFailure(result *verification.VerificationResult) bool {
	return result.Action == verification.Enforced && !result.Success
}

func (v *notaryV2Verifier) processSignature(ctx context.Context, sigBlob []byte, sigManifest notationregistry.SignatureManifest, outcome *verification.SignatureVerificationOutcome) error {

	// verify integrity first. notation will always verify integrity no matter what the signing scheme is
	envContent, integrityResult := v.verifyIntegrity(sigBlob, sigManifest, outcome)
	outcome.EnvelopeContent = envContent
	outcome.VerificationResults = append(outcome.VerificationResults, integrityResult)
	if integrityResult.Error != nil {
		return integrityResult.Error
	}

	// verify x509 trust store based authenticity
	authenticityResult := v.verifyAuthenticity(v.trustCerts, outcome)
	outcome.VerificationResults = append(outcome.VerificationResults, authenticityResult)
	if isCriticalFailure(authenticityResult) {
		return authenticityResult.Error
	}

	v.verifyX509TrustedIdentities(outcome, authenticityResult)
	if isCriticalFailure(authenticityResult) {
		return authenticityResult.Error
	}

	// verify expiry
	expiryResult := v.verifyExpiry(outcome)
	outcome.VerificationResults = append(outcome.VerificationResults, expiryResult)
	if isCriticalFailure(expiryResult) {
		return expiryResult.Error
	}

	// verify authentic timestamp
	authenticTimestampResult := v.verifyAuthenticTimestamp(outcome)
	outcome.VerificationResults = append(outcome.VerificationResults, authenticTimestampResult)
	if isCriticalFailure(authenticTimestampResult) {
		return authenticTimestampResult.Error
	}

	// verify revocation
	// check if we need to bypass the revocation check, since revocation can be skipped using a trust policy or a plugin may override the check
	//if outcome.VerificationLevel.VerificationMap[verification.Revocation] != verification.Skipped {
	// TODO perform X509 revocation check (not in RC1)
	// https://github.com/notaryproject/notation-go/issues/110
	//}

	return nil
}

func (v *notaryV2Verifier) verifyIntegrity(sigBlob []byte, sigManifest notationregistry.SignatureManifest, outcome *verification.SignatureVerificationOutcome) (*signature.EnvelopeContent, *verification.VerificationResult) {
	// parse the signature
	sigEnv, err := signature.ParseEnvelope(sigManifest.Blob.MediaType, sigBlob)
	if err != nil {
		return nil, &verification.VerificationResult{
			Success: false,
			Error:   fmt.Errorf("unable to parse the digital signature, error : %s", err),
			Type:    verification.Integrity,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Integrity],
		}
	}

	// verify integrity
	envContent, err := sigEnv.Verify()
	if err != nil {
		switch err.(type) {
		case *signature.SignatureEnvelopeNotFoundError, *signature.InvalidSignatureError, *signature.SignatureIntegrityError:
			return nil, &verification.VerificationResult{
				Success: false,
				Error:   err,
				Type:    verification.Integrity,
				Action:  outcome.VerificationLevel.VerificationMap[verification.Integrity],
			}
		default:
			// unexpected error
			return nil, &verification.VerificationResult{
				Success: false,
				Error:   err,
				Type:    verification.Integrity,
				Action:  outcome.VerificationLevel.VerificationMap[verification.Integrity],
			}
		}
	}

	if err := sig.ValidatePayloadContentType(&envContent.Payload); err != nil {
		return nil, &verification.VerificationResult{
			Success: false,
			Error:   err,
			Type:    verification.Integrity,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Integrity],
		}
	}

	// integrity has been verified successfully
	return envContent, &verification.VerificationResult{
		Success: true,
		Type:    verification.Integrity,
		Action:  outcome.VerificationLevel.VerificationMap[verification.Integrity],
	}
}

func (v *notaryV2Verifier) verifyAuthenticity(trustCerts []*x509.Certificate, outcome *verification.SignatureVerificationOutcome) *verification.VerificationResult {
	if len(trustCerts) < 1 {
		return &verification.VerificationResult{
			Success: false,
			Error:   errors.Errorf("no trusted certificates are found to verify authenticity"),
			Type:    verification.Authenticity,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Authenticity],
		}
	}
	_, err := signature.VerifyAuthenticity(&outcome.EnvelopeContent.SignerInfo, trustCerts)
	if err != nil {
		switch err.(type) {
		case *signature.SignatureAuthenticityError:
			return &verification.VerificationResult{
				Success: false,
				Error:   err,
				Type:    verification.Authenticity,
				Action:  outcome.VerificationLevel.VerificationMap[verification.Authenticity],
			}
		default:
			return &verification.VerificationResult{
				Success: false,
				Error:   errors.Wrapf(err, "authenticity verification failed"),
				Type:    verification.Authenticity,
				Action:  outcome.VerificationLevel.VerificationMap[verification.Authenticity],
			}
		}
	} else {
		return &verification.VerificationResult{
			Success: true,
			Type:    verification.Authenticity,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Authenticity],
		}
	}
}

func (v *notaryV2Verifier) verifyExpiry(outcome *verification.SignatureVerificationOutcome) *verification.VerificationResult {
	if expiry := outcome.EnvelopeContent.SignerInfo.SignedAttributes.Expiry; !expiry.IsZero() && !time.Now().Before(expiry) {
		return &verification.VerificationResult{
			Success: false,
			Error:   fmt.Errorf("digital signature has expired on %q", expiry.Format(time.RFC1123Z)),
			Type:    verification.Expiry,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Expiry],
		}
	} else {
		return &verification.VerificationResult{
			Success: true,
			Type:    verification.Expiry,
			Action:  outcome.VerificationLevel.VerificationMap[verification.Expiry],
		}
	}
}

func (v *notaryV2Verifier) verifyAuthenticTimestamp(outcome *verification.SignatureVerificationOutcome) *verification.VerificationResult {
	invalidTimestamp := false
	var err error

	if signerInfo := outcome.EnvelopeContent.SignerInfo; signerInfo.SignedAttributes.SigningScheme == signature.SigningSchemeX509 {
		// TODO verify RFC3161 TSA signature if present (not in RC1)
		// https://github.com/notaryproject/notation-go/issues/78
		if len(signerInfo.UnsignedAttributes.TimestampSignature) == 0 {
			// if there is no TSA signature, then every certificate should be valid at the time of verification
			now := time.Now()
			for _, cert := range signerInfo.CertificateChain {
				if now.Before(cert.NotBefore) {
					invalidTimestamp = true
					err = fmt.Errorf("certificate %q is not valid yet, it will be valid from %q", cert.Subject, cert.NotBefore.Format(time.RFC1123Z))
					break
				}
				if now.After(cert.NotAfter) {
					invalidTimestamp = true
					err = fmt.Errorf("certificate %q is not valid anymore, it was expired at %q", cert.Subject, cert.NotAfter.Format(time.RFC1123Z))
					break
				}
			}
		}
	} else if signerInfo.SignedAttributes.SigningScheme == signature.SigningSchemeX509SigningAuthority {
		authenticSigningTime := signerInfo.SignedAttributes.SigningTime
		// TODO use authenticSigningTime from signerInfo
		// https://github.com/notaryproject/notation-core-go/issues/38
		for _, cert := range signerInfo.CertificateChain {
			if authenticSigningTime.Before(cert.NotBefore) || authenticSigningTime.After(cert.NotAfter) {
				invalidTimestamp = true
				err = fmt.Errorf("certificate %q was not valid when the digital signature was produced at %q", cert.Subject, authenticSigningTime.Format(time.RFC1123Z))
				break
			}
		}
	}

	if invalidTimestamp {
		return &verification.VerificationResult{
			Success: false,
			Error:   err,
			Type:    verification.AuthenticTimestamp,
			Action:  outcome.VerificationLevel.VerificationMap[verification.AuthenticTimestamp],
		}
	} else {
		return &verification.VerificationResult{
			Success: true,
			Type:    verification.AuthenticTimestamp,
			Action:  outcome.VerificationLevel.VerificationMap[verification.AuthenticTimestamp],
		}
	}
}

// verifyX509TrustedIdentities verified x509 trusted identities. This functions uses the VerificationResult from x509 trust store verification and modifies it
func (v *notaryV2Verifier) verifyX509TrustedIdentities(outcome *verification.SignatureVerificationOutcome, authenticityResult *verification.VerificationResult) {
	// verify trusted identities
	err := verifyX509TrustedIdentities(v.trustedX509Identities, outcome.EnvelopeContent.SignerInfo.CertificateChain)
	if err != nil {
		authenticityResult.Success = false
		authenticityResult.Error = err
	}
}

func verifyX509TrustedIdentities(trustedX509Identities map[string]string, certs []*x509.Certificate) error {
	if len(trustedX509Identities) == 0 {
		return nil
	}

	leafCert := certs[0] // trusted identities only supported on the leaf cert

	leafCertDN, err := parseDistinguishedName(leafCert.Subject.String()) // parse the certificate subject following rfc 4514 DN syntax
	if err != nil {
		return fmt.Errorf("error while parsing the certificate subject from the digital signature. error : %q", err)
	}

	if !isSubsetDN(trustedX509Identities, leafCertDN) {
		return fmt.Errorf("failed to match X.509 trusted identity %q in %q", trustedX509Identities, leafCertDN)
	}

	return nil
}

// parseDistinguishedName parses a DN name and validates Notary V2 rules
// C=US, ST=WA, L=Seattle, O=wabbit-networks.io, OU=Finance, CN=SecureBuilder
func parseDistinguishedName(name string) (map[string]string, error) {
	mandatoryFields := []string{"C", "ST", "O"}
	attrKeyValue := make(map[string]string)
	dn, err := ldapv3.ParseDN(name)

	if err != nil {
		return nil, fmt.Errorf("distinguished name (DN) %q is not valid, it must contain 'C', 'ST', and 'O' RDN attributes at a minimum, and follow RFC 4514 standard", name)
	}

	for _, rdn := range dn.RDNs {

		// multi-valued RDNs are not supported (TODO: add spec reference here)
		if len(rdn.Attributes) > 1 {
			return nil, fmt.Errorf("distinguished name (DN) %q has multi-valued RDN attributes, remove multi-valued RDN attributes as they are not supported", name)
		}
		for _, attribute := range rdn.Attributes {
			if attrKeyValue[attribute.Type] == "" {
				attrKeyValue[attribute.Type] = attribute.Value
			} else {
				return nil, fmt.Errorf("distinguished name (DN) %q has duplicate RDN attribute for %q, DN can only have unique RDN attributes", name, attribute.Type)
			}
		}
	}

	// Verify mandatory fields are present
	for _, field := range mandatoryFields {
		if attrKeyValue[field] == "" {
			return nil, fmt.Errorf("distinguished name (DN) %q has no mandatory RDN attribute for %q, it must contain 'C', 'ST', and 'O' RDN attributes at a minimum", name, field)
		}
	}
	// No errors
	return attrKeyValue, nil
}

// isSubsetDN returns true if dn1 is a subset of dn2 i.e. every key/value pair of dn1 has a matching key/value pair in dn2, otherwise returns false
func isSubsetDN(dn1 map[string]string, dn2 map[string]string) bool {
	for key := range dn1 {
		if dn1[key] != dn2[key] {
			return false
		}
	}
	return true
}

func (v *notaryV2Verifier) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	return nil, errors.Errorf("not implemented")
}
