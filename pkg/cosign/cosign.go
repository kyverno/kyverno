package cosign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/images"
	"github.com/kyverno/kyverno/pkg/tracing"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/cosign/attestation"
	"github.com/sigstore/cosign/v2/pkg/oci"
	"github.com/sigstore/cosign/v2/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"
	rekorclient "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
	"github.com/sigstore/sigstore/pkg/tuf"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
)

var signatureAlgorithmMap = map[string]crypto.Hash{
	"":       crypto.SHA256,
	"sha224": crypto.SHA224,
	"sha256": crypto.SHA256,
	"sha384": crypto.SHA384,
	"sha512": crypto.SHA512,
}

func NewVerifier() images.ImageVerifier {
	return &cosignVerifier{}
}

type cosignVerifier struct{}

func (v *cosignVerifier) VerifySignature(ctx context.Context, opts images.Options) (*images.Response, error) {
	ref, err := name.ParseReference(opts.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image %s", opts.ImageRef)
	}

	signatures, bundleVerified, err := tracing.ChildSpan3(
		ctx,
		"",
		"VERIFY IMG SIGS",
		func(ctx context.Context, span trace.Span) ([]oci.Signature, bool, error) {
			cosignOpts, err := buildCosignOptions(ctx, opts)
			if err != nil {
				return nil, false, err
			}
			return client.VerifyImageSignatures(ctx, ref, cosignOpts)
		},
	)
	if err != nil {
		logger.Info("image verification failed", "error", err.Error())
		return nil, err
	}

	logger.V(3).Info("verified image", "count", len(signatures), "bundleVerified", bundleVerified)
	payload, err := extractPayload(signatures)
	if err != nil {
		return nil, err
	}

	if err := matchSignatures(signatures, opts.Subject, opts.Issuer, opts.AdditionalExtensions); err != nil {
		return nil, err
	}

	err = checkAnnotations(payload, opts.Annotations)
	if err != nil {
		return nil, err
	}

	var digest string
	if opts.Type == "" {
		digest, err = extractDigest(opts.ImageRef, payload)
		if err != nil {
			return nil, err
		}
	}

	return &images.Response{Digest: digest}, nil
}

func buildCosignOptions(ctx context.Context, opts images.Options) (*cosign.CheckOpts, error) {
	var remoteOpts []remote.Option
	var err error

	cosignRemoteOpts, err := opts.Client.BuildCosignRemoteOption(ctx)
	if err != nil {
		return nil, fmt.Errorf("constructing cosign remote options: %w", err)
	}
	remoteOpts = append(remoteOpts, cosignRemoteOpts)
	cosignOpts := &cosign.CheckOpts{
		Annotations:        map[string]interface{}{},
		RegistryClientOpts: remoteOpts,
	}

	if opts.FetchAttestations {
		cosignOpts.ClaimVerifier = cosign.IntotoSubjectClaimVerifier
	} else {
		cosignOpts.ClaimVerifier = cosign.SimpleClaimVerifier
	}

	if opts.Roots != "" {
		cp, err := loadCertPool([]byte(opts.Roots))
		if err != nil {
			return nil, fmt.Errorf("failed to load Root certificates: %w", err)
		}
		cosignOpts.RootCerts = cp
	}

	if opts.Key != "" {
		if strings.HasPrefix(strings.TrimSpace(opts.Key), "-----BEGIN PUBLIC KEY-----") {
			if signatureAlgorithm, ok := signatureAlgorithmMap[opts.SignatureAlgorithm]; ok {
				cosignOpts.SigVerifier, err = decodePEM([]byte(opts.Key), signatureAlgorithm)
				if err != nil {
					return nil, fmt.Errorf("failed to load public key from PEM: %w", err)
				}
			} else {
				return nil, fmt.Errorf("invalid signature algorithm provided %s", opts.SignatureAlgorithm)
			}
		} else {
			// this supports Kubernetes secrets and KMS
			cosignOpts.SigVerifier, err = sigs.PublicKeyFromKeyRef(ctx, opts.Key)
			if err != nil {
				return nil, fmt.Errorf("failed to load public key from %s: %w", opts.Key, err)
			}
		}
	} else {
		if opts.Cert != "" {
			// load cert and optionally a cert chain as a verifier
			cert, err := loadCert([]byte(opts.Cert))
			if err != nil {
				return nil, fmt.Errorf("failed to load certificate from %s: %w", opts.Cert, err)
			}

			if opts.CertChain == "" {
				cosignOpts.SigVerifier, err = signature.LoadVerifier(cert.PublicKey, crypto.SHA256)
				if err != nil {
					return nil, fmt.Errorf("failed to load signature from certificate: %w", err)
				}
			} else {
				// Verify certificate with chain
				chain, err := loadCertChain([]byte(opts.CertChain))
				if err != nil {
					return nil, fmt.Errorf("failed to load load certificate chain: %w", err)
				}
				cosignOpts.SigVerifier, err = cosign.ValidateAndUnpackCertWithChain(cert, chain, cosignOpts)
				if err != nil {
					return nil, fmt.Errorf("failed to load validate certificate chain: %w", err)
				}
			}
		} else if opts.CertChain != "" {
			// load cert chain as roots
			cp, err := loadCertPool([]byte(opts.CertChain))
			if err != nil {
				return nil, fmt.Errorf("failed to load certificates: %w", err)
			}
			cosignOpts.RootCerts = cp
		} else {
			// if key, cert, and roots are not provided, default to Fulcio roots
			if cosignOpts.RootCerts == nil {
				roots, err := fulcioroots.Get()
				if err != nil {
					return nil, fmt.Errorf("failed to get roots from fulcio: %w", err)
				}
				cosignOpts.RootCerts = roots
				if cosignOpts.RootCerts == nil {
					return nil, fmt.Errorf("failed to initialize roots")
				}
			}
		}
	}

	cosignOpts.IgnoreTlog = opts.IgnoreTlog
	if !opts.IgnoreTlog {
		cosignOpts.RekorClient, err = rekorclient.GetRekorClient(opts.RekorURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create Rekor client from URL %s: %w", opts.RekorURL, err)
		}

		cosignOpts.RekorPubKeys, err = getRekorPubs(ctx, opts.RekorPubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load Rekor public keys: %w", err)
		}
	}

	cosignOpts.IgnoreSCT = opts.IgnoreSCT
	if !opts.IgnoreSCT {
		cosignOpts.CTLogPubKeys, err = getCTLogPubs(ctx, opts.CTLogsPubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load CTLogs public keys: %w", err)
		}
	}

	if opts.Repository != "" {
		signatureRepo, err := name.NewRepository(opts.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to parse signature repository %s: %w", opts.Repository, err)
		}

		cosignOpts.RegistryClientOpts = append(cosignOpts.RegistryClientOpts, remote.WithTargetRepository(signatureRepo))
	}

	return cosignOpts, nil
}

func loadCertPool(roots []byte) (*x509.CertPool, error) {
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(roots) {
		return nil, fmt.Errorf("error creating root cert pool")
	}

	return cp, nil
}

func loadCert(pem []byte) (*x509.Certificate, error) {
	var out []byte
	out, err := base64.StdEncoding.DecodeString(string(pem))
	if err != nil {
		// not a base64
		out = pem
	}

	certs, err := cryptoutils.UnmarshalCertificatesFromPEM(out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate from PEM format: %w", err)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certs found in pem file")
	}
	return certs[0], nil
}

func loadCertChain(pem []byte) ([]*x509.Certificate, error) {
	return cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(pem))
}

func (v *cosignVerifier) FetchAttestations(ctx context.Context, opts images.Options) (*images.Response, error) {
	cosignOpts, err := buildCosignOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	signatures, bundleVerified, err := tracing.ChildSpan3(
		ctx,
		"",
		"VERIFY IMG ATTESTATIONS",
		func(ctx context.Context, span trace.Span) (checkedAttestations []oci.Signature, bundleVerified bool, err error) {
			ref, err := name.ParseReference(opts.ImageRef)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse image: %w", err)
			}
			return client.VerifyImageAttestations(ctx, ref, cosignOpts)
		},
	)
	if err != nil {
		msg := err.Error()
		logger.Info("failed to fetch attestations", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return nil, fmt.Errorf("not found")
		}

		return nil, err
	}

	payload, err := extractPayload(signatures)
	if err != nil {
		return nil, err
	}

	for _, signature := range signatures {
		match, predicateType, err := matchType(signature, opts.Type)
		if err != nil {
			return nil, err
		}

		if !match {
			logger.V(4).Info("type doesn't match, continue", "expected", opts.Type, "received", predicateType)
			continue
		}

		if err := matchSignatures([]oci.Signature{signature}, opts.Subject, opts.Issuer, opts.AdditionalExtensions); err != nil {
			return nil, err
		}
	}

	err = checkAnnotations(payload, opts.Annotations)
	if err != nil {
		return nil, err
	}

	logger.V(3).Info("verified images", "signatures", len(signatures), "bundleVerified", bundleVerified)
	inTotoStatements, digest, err := decodeStatements(signatures)
	if err != nil {
		return nil, err
	}

	return &images.Response{Digest: digest, Statements: inTotoStatements}, nil
}

func matchType(sig oci.Signature, expectedType string) (bool, string, error) {
	if expectedType != "" {
		statement, _, err := decodeStatement(sig)
		if err != nil {
			return false, "", fmt.Errorf("failed to decode type: %w", err)
		}

		if pType, ok := statement["type"]; ok {
			if pType.(string) == expectedType {
				return true, pType.(string), nil
			}
		}
	}
	return false, "", nil
}

func decodeStatements(sigs []oci.Signature) ([]map[string]interface{}, string, error) {
	if len(sigs) == 0 {
		return []map[string]interface{}{}, "", nil
	}

	var digest string
	var statement map[string]interface{}
	decodedStatements := make([]map[string]interface{}, len(sigs))
	for i, sig := range sigs {
		var err error
		statement, digest, err = decodeStatement(sig)
		if err != nil {
			return nil, "", err
		}

		decodedStatements[i] = statement
	}

	return decodedStatements, digest, nil
}

func decodeStatement(sig oci.Signature) (map[string]interface{}, string, error) {
	var digest string

	pld, err := sig.Payload()
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode payload: %w", err)
	}

	sci := payload.SimpleContainerImage{}
	if err := json.Unmarshal(pld, &sci); err != nil {
		return nil, "", fmt.Errorf("error decoding the payload: %w", err)
	}

	if d := sci.Critical.Image.DockerManifestDigest; d != "" {
		digest = d
	}

	data := make(map[string]interface{})
	if err := json.Unmarshal(pld, &data); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal JSON payload: %v: %w", sig, err)
	}

	if dataPayload, ok := data["payload"]; !ok {
		return nil, "", fmt.Errorf("missing payload in %v", data)
	} else {
		decodedStatement, err := decodePayload(dataPayload.(string))
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode statement %s: %w", string(pld), err)
		}
		decodedStatement["type"] = decodedStatement["predicateType"]

		return decodedStatement, digest, nil
	}
}

func decodePayload(payloadBase64 string) (map[string]interface{}, error) {
	statementRaw, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode payload for %v: %w", statementRaw, err)
	}

	var statement in_toto.Statement
	if err := json.Unmarshal(statementRaw, &statement); err != nil {
		return nil, err
	}

	if statement.Type != attestation.CosignCustomProvenanceV01 {
		// This assumes that the following statements are JSON objects:
		// - in_toto.PredicateSLSAProvenanceV01
		// - in_toto.PredicateLinkV1
		// - in_toto.PredicateSPDX
		// any other custom predicate
		return datautils.ToMap(statement)
	}

	return decodeCosignCustomProvenanceV01(statement)
}

func decodeCosignCustomProvenanceV01(statement in_toto.Statement) (map[string]interface{}, error) {
	if statement.Type != attestation.CosignCustomProvenanceV01 {
		return nil, fmt.Errorf("invalid statement type %s", attestation.CosignCustomProvenanceV01)
	}

	predicate, ok := statement.Predicate.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to decode CosignCustomProvenanceV01")
	}

	cosignPredicateData := predicate["Data"]
	if cosignPredicateData == nil {
		return nil, fmt.Errorf("missing predicate in CosignCustomProvenanceV01")
	}

	// attempt to parse as a JSON object type
	data, err := stringToJSONMap(cosignPredicateData)
	if err == nil {
		predicate["Data"] = data
		statement.Predicate = predicate
	}

	return datautils.ToMap(statement)
}

func stringToJSONMap(i interface{}) (map[string]interface{}, error) {
	s, ok := i.(string)
	if !ok {
		return nil, fmt.Errorf("expected string type")
	}

	data := map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return data, nil
}

func decodePEM(raw []byte, signatureAlgorithm crypto.Hash) (signature.Verifier, error) {
	// PEM encoded file.
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("pem to public key: %w", err)
	}

	return signature.LoadVerifier(pubKey, signatureAlgorithm)
}

func extractPayload(verified []oci.Signature) ([]payload.SimpleContainerImage, error) {
	var sigPayloads []payload.SimpleContainerImage
	for _, sig := range verified {
		pld, err := sig.Payload()
		if err != nil {
			return nil, fmt.Errorf("failed to get payload: %w", err)
		}

		sci := payload.SimpleContainerImage{}
		if err := json.Unmarshal(pld, &sci); err != nil {
			return nil, fmt.Errorf("error decoding the payload: %w", err)
		}

		sigPayloads = append(sigPayloads, sci)
	}
	return sigPayloads, nil
}

func extractDigest(imgRef string, payload []payload.SimpleContainerImage) (string, error) {
	for _, p := range payload {
		if digest := p.Critical.Image.DockerManifestDigest; digest != "" {
			return digest, nil
		} else {
			return "", fmt.Errorf("failed to extract image digest from signature payload for %s", imgRef)
		}
	}
	return "", fmt.Errorf("digest not found for %s", imgRef)
}

func matchSignatures(signatures []oci.Signature, subject, issuer string, extensions map[string]string) error {
	if subject == "" && issuer == "" && len(extensions) == 0 {
		return nil
	}

	var errs []error
	for _, sig := range signatures {
		cert, err := sig.Cert()
		if err != nil {
			return fmt.Errorf("failed to read certificate: %w", err)
		}

		if cert == nil {
			return fmt.Errorf("certificate not found")
		}

		if err := matchCertificateData(cert, subject, issuer, extensions); err != nil {
			errs = append(errs, err)
		} else {
			// only one signature certificate needs to match the required subject, issuer, and extensions
			return nil
		}
	}

	if len(errs) > 0 {
		err := multierr.Combine(errs...)
		return err
	}

	return fmt.Errorf("invalid signature")
}

func matchCertificateData(cert *x509.Certificate, subject, issuer string, extensions map[string]string) error {
	if subject != "" {
		s := sigs.CertSubject(cert)
		if !wildcard.Match(subject, s) {
			return fmt.Errorf("subject mismatch: expected %s, received %s", subject, s)
		}
	}

	if err := matchExtensions(cert, issuer, extensions); err != nil {
		return err
	}

	return nil
}

func matchExtensions(cert *x509.Certificate, issuer string, extensions map[string]string) error {
	ce := cosign.CertExtensions{Cert: cert}

	if issuer != "" {
		val := ce.GetIssuer()
		if !wildcard.Match(issuer, val) {
			return fmt.Errorf("issuer mismatch: expected %s, received %s", issuer, val)
		}
	}

	for requiredKey, requiredValue := range extensions {
		val, err := extractCertExtensionValue(requiredKey, ce)
		if err != nil {
			return err
		}

		if !wildcard.Match(requiredValue, val) {
			return fmt.Errorf("extension mismatch: expected %s for key %s, received %s", requiredValue, requiredKey, val)
		}
	}

	return nil
}

func extractCertExtensionValue(key string, ce cosign.CertExtensions) (string, error) {
	switch key {
	case cosign.CertExtensionOIDCIssuer, cosign.CertExtensionMap[cosign.CertExtensionOIDCIssuer]:
		return ce.GetIssuer(), nil
	case cosign.CertExtensionGithubWorkflowTrigger, cosign.CertExtensionMap[cosign.CertExtensionGithubWorkflowTrigger]:
		return ce.GetCertExtensionGithubWorkflowTrigger(), nil
	case cosign.CertExtensionGithubWorkflowSha, cosign.CertExtensionMap[cosign.CertExtensionGithubWorkflowSha]:
		return ce.GetExtensionGithubWorkflowSha(), nil
	case cosign.CertExtensionGithubWorkflowName, cosign.CertExtensionMap[cosign.CertExtensionGithubWorkflowName]:
		return ce.GetCertExtensionGithubWorkflowName(), nil
	case cosign.CertExtensionGithubWorkflowRepository, cosign.CertExtensionMap[cosign.CertExtensionGithubWorkflowRepository]:
		return ce.GetCertExtensionGithubWorkflowRepository(), nil
	case cosign.CertExtensionGithubWorkflowRef, cosign.CertExtensionMap[cosign.CertExtensionGithubWorkflowRef]:
		return ce.GetCertExtensionGithubWorkflowRef(), nil
	default:
		return "", fmt.Errorf("invalid certificate extension %s", key)
	}
}

func checkAnnotations(payload []payload.SimpleContainerImage, annotations map[string]string) error {
	for _, p := range payload {
		for key, val := range annotations {
			if val != p.Optional[key] {
				return fmt.Errorf("annotations mismatch: %s does not match expected value %s for key %s",
					p.Optional[key], val, key)
			}
		}
	}
	return nil
}

func getRekorPubs(ctx context.Context, rekorPubKey string) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if rekorPubKey == "" {
		return cosign.GetRekorPubs(ctx)
	}

	publicKeys := cosign.NewTrustedTransparencyLogPubKeys()
	if err := publicKeys.AddTransparencyLogPubKey([]byte(rekorPubKey), tuf.Active); err != nil {
		return nil, fmt.Errorf("failed to get rekor public keys: %w", err)
	}
	return &publicKeys, nil
}

func getCTLogPubs(ctx context.Context, ctlogPubKey string) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if ctlogPubKey == "" {
		return cosign.GetCTLogPubs(ctx)
	}

	publicKeys := cosign.NewTrustedTransparencyLogPubKeys()
	if err := publicKeys.AddTransparencyLogPubKey([]byte(ctlogPubKey), tuf.Active); err != nil {
		return nil, fmt.Errorf("failed to get transparency log public keys: %w", err)
	}
	return &publicKeys, nil
}
