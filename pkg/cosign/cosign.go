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
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/in-toto/in-toto-golang/in_toto"
	wildcard "github.com/kyverno/go-wildcard"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/cmd/cosign/cli/rekor"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/cosign/attestation"
	"github.com/sigstore/cosign/pkg/oci"
	"github.com/sigstore/cosign/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/pkg/signature"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

// ImageSignatureRepository is an alternate signature repository
var ImageSignatureRepository string

type Options struct {
	ImageRef             string
	FetchAttestations    bool
	Key                  string
	Cert                 string
	CertChain            string
	Roots                string
	Subject              string
	Issuer               string
	AdditionalExtensions map[string]string
	Annotations          map[string]string
	Repository           string
	RekorURL             string
}

type Response struct {
	Digest     string
	Statements []map[string]interface{}
}

func Verify(opts Options) (*Response, error) {
	if opts.FetchAttestations {
		return fetchAttestations(opts)
	} else {
		return verifySignature(opts)
	}
}

// verifySignature verifies that the image has the expected signatures
func verifySignature(opts Options) (*Response, error) {
	ref, err := name.ParseReference(opts.ImageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image %s", opts.ImageRef)
	}

	cosignOpts, err := buildCosignOptions(opts)
	if err != nil {
		return nil, err
	}

	signatures, bundleVerified, err := client.VerifyImageSignatures(context.Background(), ref, cosignOpts)
	if err != nil {
		logger.Info("image verification failed", "error", err.Error())
		return nil, err
	}

	logger.V(3).Info("verified image", "count", len(signatures), "bundleVerified", bundleVerified)
	payload, err := extractPayload(signatures)
	if err != nil {
		return nil, err
	}

	if err := matchSubjectAndIssuer(signatures, opts.Subject, opts.Issuer); err != nil {
		return nil, err
	}

	if err := matchExtensions(signatures, opts.AdditionalExtensions); err != nil {
		return nil, err
	}

	err = checkAnnotations(payload, opts.Annotations)
	if err != nil {
		return nil, err
	}

	digest, err := extractDigest(opts.ImageRef, payload)
	if err != nil {
		return nil, err
	}

	return &Response{Digest: digest}, nil
}

func buildCosignOptions(opts Options) (*cosign.CheckOpts, error) {
	var remoteOpts []remote.Option
	var err error
	ro := options.RegistryOptions{}
	remoteOpts, err = ro.ClientOpts(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "constructing client options")
	}

	remoteOpts = append(remoteOpts, remote.WithRemoteOptions(gcrremote.WithAuthFromKeychain(registryclient.DefaultKeychain)))
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
			return nil, errors.Wrap(err, "failed to load Root certificates")
		}
		cosignOpts.RootCerts = cp
	}

	if opts.Key != "" {
		if strings.HasPrefix(opts.Key, "-----BEGIN PUBLIC KEY-----") {
			cosignOpts.SigVerifier, err = decodePEM([]byte(opts.Key))
			if err != nil {
				return nil, errors.Wrap(err, "failed to load public key from PEM")
			}
		} else {
			// this supports Kubernetes secrets and KMS
			cosignOpts.SigVerifier, err = sigs.PublicKeyFromKeyRef(context.Background(), opts.Key)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to load public key from %s", opts.Key)
			}
		}
	} else {
		if opts.Cert != "" {
			// load cert and optionally a cert chain as a verifier
			cert, err := loadCert([]byte(opts.Cert))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to load certificate from %s", string(opts.Cert))
			}

			if opts.CertChain == "" {
				cosignOpts.SigVerifier, err = signature.LoadVerifier(cert.PublicKey, crypto.SHA256)
				if err != nil {
					return nil, errors.Wrap(err, "failed to load signature from certificate")
				}
			} else {
				// Verify certificate with chain
				chain, err := loadCertChain([]byte(opts.CertChain))
				if err != nil {
					return nil, errors.Wrap(err, "failed to load load certificate chain")
				}
				cosignOpts.SigVerifier, err = cosign.ValidateAndUnpackCertWithChain(cert, chain, cosignOpts)
				if err != nil {
					return nil, errors.Wrap(err, "failed to load validate certificate chain")
				}
			}
		} else if opts.CertChain != "" {
			// load cert chain as roots
			cp, err := loadCertPool([]byte(opts.CertChain))
			if err != nil {
				return nil, errors.Wrap(err, "failed to load certificates")
			}
			cosignOpts.RootCerts = cp
		} else {
			// if key, cert, and roots are not provided, default to Fulcio roots
			if cosignOpts.RootCerts == nil {
				cosignOpts.RootCerts = fulcio.GetRoots()
				if cosignOpts.RootCerts == nil {
					return nil, fmt.Errorf("failed to initialize roots")
				}
			}
		}
	}

	if opts.RekorURL != "" {
		cosignOpts.RekorClient, err = rekor.NewClient(opts.RekorURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create Rekor client from URL %s", opts.RekorURL)
		}
	}

	if opts.Repository != "" {
		signatureRepo, err := name.NewRepository(opts.Repository)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse signature repository %s", opts.Repository)
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
		return nil, errors.Wrap(err, "failed to unmarshal certificate from PEM format")
	}
	if len(certs) == 0 {
		return nil, errors.New("no certs found in pem file")
	}
	return certs[0], nil
}

func loadCertChain(pem []byte) ([]*x509.Certificate, error) {
	return cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(pem))
}

// fetchAttestations retrieves signed attestations and decodes them into in-toto statements
// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
func fetchAttestations(opts Options) (*Response, error) {
	cosignOpts, err := buildCosignOptions(opts)
	if err != nil {
		return nil, err
	}

	ref, err := name.ParseReference(opts.ImageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse image")
	}

	signatures, bundleVerified, err := client.VerifyImageAttestations(context.Background(), ref, cosignOpts)
	if err != nil {
		msg := err.Error()
		logger.Info("failed to fetch attestations", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return nil, errors.Wrap(fmt.Errorf("not found"), "")
		}

		return nil, err
	}

	logger.V(3).Info("verified images", "signatures", len(signatures), "bundleVerified", bundleVerified)
	inTotoStatements, digest, err := decodeStatements(signatures)
	if err != nil {
		return nil, err
	}

	return &Response{Digest: digest, Statements: inTotoStatements}, nil
}

func decodeStatements(sigs []oci.Signature) ([]map[string]interface{}, string, error) {
	if len(sigs) == 0 {
		return []map[string]interface{}{}, "", nil
	}

	var digest string
	decodedStatements := make([]map[string]interface{}, len(sigs))
	for i, sig := range sigs {
		pld, err := sig.Payload()
		if err != nil {
			return nil, "", errors.Wrap(err, "failed to decode payload")
		}

		sci := payload.SimpleContainerImage{}
		if err := json.Unmarshal(pld, &sci); err != nil {
			return nil, "", errors.Wrap(err, "error decoding the payload")
		}

		if d := sci.Critical.Image.DockerManifestDigest; d != "" {
			digest = d
		}

		data := make(map[string]interface{})
		if err := json.Unmarshal(pld, &data); err != nil {
			return nil, "", errors.Wrapf(err, "failed to unmarshal JSON payload: %v", sig)
		}

		if dataPayload, ok := data["payload"]; !ok {
			return nil, "", fmt.Errorf("missing payload in %v", data)
		} else {
			decodedStatement, err := decodeStatement(dataPayload.(string))
			if err != nil {
				return nil, "", errors.Wrapf(err, "failed to decode statement %s", string(pld))
			}

			decodedStatements[i] = decodedStatement
		}
	}

	return decodedStatements, digest, nil
}

func decodeStatement(payloadBase64 string) (map[string]interface{}, error) {
	statementRaw, err := base64.StdEncoding.DecodeString(payloadBase64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to base64 decode payload for %v", statementRaw)
	}

	var statement in_toto.Statement
	if err := json.Unmarshal(statementRaw, &statement); err != nil {
		return nil, err
	}

	if statement.PredicateType != attestation.CosignCustomProvenanceV01 {
		// This assumes that the following statements are JSON objects:
		// - in_toto.PredicateSLSAProvenanceV01
		// - in_toto.PredicateLinkV1
		// - in_toto.PredicateSPDX
		// any other custom predicate
		return utils.ToMap(statement)
	}

	return decodeCosignCustomProvenanceV01(statement)
}

func decodeCosignCustomProvenanceV01(statement in_toto.Statement) (map[string]interface{}, error) {
	if statement.PredicateType != attestation.CosignCustomProvenanceV01 {
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

	return utils.ToMap(statement)
}

func stringToJSONMap(i interface{}) (map[string]interface{}, error) {
	s, ok := i.(string)
	if !ok {
		return nil, fmt.Errorf("expected string type")
	}

	var data = map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %s", err.Error())
	}

	return data, nil
}

func decodePEM(raw []byte) (signature.Verifier, error) {
	// PEM encoded file.
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, errors.Wrap(err, "pem to public key")
	}

	return signature.LoadVerifier(pubKey, crypto.SHA256)
}

func extractPayload(verified []oci.Signature) ([]payload.SimpleContainerImage, error) {
	var sigPayloads []payload.SimpleContainerImage
	for _, sig := range verified {
		pld, err := sig.Payload()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get payload")
		}

		sci := payload.SimpleContainerImage{}
		if err := json.Unmarshal(pld, &sci); err != nil {
			return nil, errors.Wrap(err, "error decoding the payload")
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
			logger.Info("failed to extract image digest from verification response", "image", imgRef, "payload", p)
			return "", fmt.Errorf("unknown image response for " + imgRef)
		}
	}
	return "", fmt.Errorf("digest not found for " + imgRef)
}

func matchSubjectAndIssuer(signatures []oci.Signature, subject, issuer string) error {
	if subject == "" && issuer == "" {
		return nil
	}
	var s string
	for _, sig := range signatures {
		cert, err := sig.Cert()
		if err != nil {
			return errors.Wrap(err, "failed to read certificate")
		}

		if cert == nil {
			return errors.Wrap(err, "certificate not found")
		}

		s = sigs.CertSubject(cert)
		i := sigs.CertIssuerExtension(cert)
		if subject == "" || wildcard.Match(subject, s) {
			if issuer == "" || (issuer == i) {
				return nil
			} else {
				return fmt.Errorf("issuer mismatch: expected %s, got %s", i, issuer)
			}
		}
	}

	return fmt.Errorf("subject mismatch: expected %s, got %s", s, subject)
}

func matchExtensions(signatures []oci.Signature, requiredExtensions map[string]string) error {
	if len(requiredExtensions) == 0 {
		return nil
	}

	for _, sig := range signatures {
		cert, err := sig.Cert()
		if err != nil {
			return errors.Wrap(err, "failed to read certificate")
		}

		if cert == nil {
			return errors.Wrap(err, "certificate not found")
		}

		// This will return a map which consists of readable extension-names as keys
		// or the raw extensionIDs as fallback and its values.
		certExtensions := sigs.CertExtensions(cert)
		for requiredKey, requiredValue := range requiredExtensions {
			certValue, ok := certExtensions[requiredKey]
			if !ok {
				// "requiredKey" seems to be an extensionID, try to resolve its human readable name
				readableName, ok := sigs.CertExtensionMap[requiredKey]
				if !ok {
					return fmt.Errorf("key %s not present", requiredKey)
				}

				certValue, ok = certExtensions[readableName]
				if !ok {
					return fmt.Errorf("key %s (%s) not present", requiredKey, readableName)
				}
			}

			if requiredValue != "" && !wildcard.Match(requiredValue, certValue) {
				return fmt.Errorf("extension mismatch: expected %s for key %s, got %s", requiredValue, requiredKey, certValue)
			}
		}
	}

	return nil
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
