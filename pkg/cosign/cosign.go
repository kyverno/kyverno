package cosign

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/sigstore/cosign/cmd/cosign/cli/rekor"
	"strings"

	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/pkg/oci/remote"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/kyverno/kyverno/pkg/engine/common"
	"github.com/minio/pkg/wildcard"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/cosign/attestation"
	"github.com/sigstore/cosign/pkg/oci"
	sigs "github.com/sigstore/cosign/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

// ImageSignatureRepository is an alternate signature repository
var ImageSignatureRepository string

type Options struct {
	ImageRef    string
	Key         string
	Roots       []byte
	Subject     string
	Issuer      string
	Annotations map[string]string
	Repository  string
	Log         logr.Logger
}

// VerifySignature verifies that the image has the expected key
func VerifySignature(opts Options) (digest string, err error) {
	log := opts.Log
	ctx := context.Background()
	var remoteOpts []remote.Option
	ro := options.RegistryOptions{}
	remoteOpts, err = ro.ClientOpts(ctx)
	if err != nil {
		return "", errors.Wrap(err, "constructing client options")
	}

	cosignOpts := &cosign.CheckOpts{
		Annotations:        map[string]interface{}{},
		RegistryClientOpts: remoteOpts,
		ClaimVerifier:      cosign.SimpleClaimVerifier,
	}

	if opts.Key != "" {
		if strings.HasPrefix(opts.Key, "-----BEGIN PUBLIC KEY-----") {
			cosignOpts.SigVerifier, err = decodePEM([]byte(opts.Key))
		} else {
			cosignOpts.SigVerifier, err = sigs.PublicKeyFromKeyRef(ctx, opts.Key)
		}
	} else {
		cosignOpts.CertEmail = ""
		cosignOpts.RootCerts, err = getFulcioRoots(opts.Roots)
		if err == nil {
			cosignOpts.RekorClient, err = rekor.NewClient("https://rekor.sigstore.dev")
		}
	}

	if err != nil {
		return "", errors.Wrap(err, "loading credentials")
	}

	if opts.Repository != "" {
		signatureRepo, err := name.NewRepository(opts.Repository)
		if err != nil {
			return "", errors.Wrapf(err, "failed to parse signature repository %s", opts.Repository)
		}

		cosignOpts.RegistryClientOpts = append(cosignOpts.RegistryClientOpts, remote.WithTargetRepository(signatureRepo))
	}

	ref, err := name.ParseReference(opts.ImageRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image")
	}

	signatures, bundleVerified, err := client.VerifyImageSignatures(ctx, ref, cosignOpts)
	if err != nil {
		msg := err.Error()
		log.Info("image verification failed", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return "", fmt.Errorf("signature not found")
		} else if strings.Contains(msg, "no matching signatures") {
			return "", fmt.Errorf("signature mismatch")
		}

		return "", err
	}

	log.V(3).Info("verified image", "count", len(signatures), "bundleVerified", bundleVerified)
	pld, err := extractPayload(signatures)
	if err != nil {
		return "", errors.Wrap(err, "failed to get pld")
	}

	if opts.Issuer != "" {
		issuer, err := extractIssuer(opts.ImageRef, pld, log)
		if err == nil && (issuer != opts.Issuer) {
			return "", errors.Wrap(err, "issuer mismatch")
		}

		return "", errors.Wrap(err, "issuer not found")
	}

	if opts.Subject != "" {
		subject, err := extractSubject(opts.ImageRef, pld, log)
		if err == nil && wildcard.Match(opts.Subject, subject) {
			return "", errors.Wrap(err, "subject mismatch")
		}

		return "", errors.Wrap(err, "subject not found")
	}

	err = checkAnnotations(pld, opts.Annotations)
	if err != nil {
		return "", errors.Wrap(err, "annotation mismatch")
	}

	digest, err = extractDigest(opts.ImageRef, pld, log)
	if err != nil {
		return "", errors.Wrap(err, "failed to get digest")
	}

	return digest, nil
}

func getFulcioRoots(roots []byte) (*x509.CertPool, error) {
	if len(roots) == 0 {
		return fulcio.GetRoots(), nil
	}

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(roots) {
		return nil, fmt.Errorf("error creating root cert pool")
	}

	return cp, nil
}

// FetchAttestations retrieves signed attestations and decodes them into in-toto statements
// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
func FetchAttestations(imageRef string, key string, repository string, log logr.Logger) ([]map[string]interface{}, error) {
	ctx := context.Background()
	var pubKey signature.Verifier
	var err error

	if strings.HasPrefix(key, "-----BEGIN PUBLIC KEY-----") {
		pubKey, err = decodePEM([]byte(key))
		if err != nil {
			return nil, errors.Wrap(err, "decode pem")
		}
	} else {
		pubKey, err = sigs.PublicKeyFromKeyRef(ctx, key)
		if err != nil {
			return nil, errors.Wrap(err, "loading public key")
		}
	}

	var opts []remote.Option
	ro := options.RegistryOptions{}

	opts, err = ro.ClientOpts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "constructing client options")
	}

	if repository != "" {
		signatureRepo, err := name.NewRepository(repository)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse signature repository %s", repository)
		}
		opts = append(opts, remote.WithTargetRepository(signatureRepo))
	}

	cosignOpts := &cosign.CheckOpts{
		ClaimVerifier:      cosign.IntotoSubjectClaimVerifier,
		SigVerifier:        pubKey,
		RegistryClientOpts: opts,
	}

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse image")
	}

	signatures, bundleVerified, err := client.VerifyImageAttestations(context.Background(), ref, cosignOpts)
	if err != nil {
		msg := err.Error()
		log.Info("failed to fetch attestations", "error", msg)
		if strings.Contains(msg, "MANIFEST_UNKNOWN: manifest unknown") {
			return nil, fmt.Errorf("not found")
		}

		return nil, err
	}

	log.V(3).Info("verified images", "count", len(signatures), "bundleVerified", bundleVerified)
	inTotoStatements, err := decodeStatements(signatures)
	if err != nil {
		return nil, err
	}

	return inTotoStatements, nil
}

func decodeStatements(sigs []oci.Signature) ([]map[string]interface{}, error) {
	if len(sigs) == 0 {
		return []map[string]interface{}{}, nil
	}

	decodedStatements := make([]map[string]interface{}, len(sigs))
	for i, sig := range sigs {
		pld, err := sig.Payload()
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode payload")
		}

		data := make(map[string]interface{})
		if err := json.Unmarshal(pld, &data); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal JSON payload: %v", sig)
		}

		if dataPayload, ok := data["payload"]; !ok  {
			return nil, fmt.Errorf("missing payload in %v", data)
		} else {
			decodedStatement, err := decodeStatement(dataPayload.(string))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to decode statement %s", string(pld))
			}

			decodedStatements[i] = decodedStatement
		}
	}

	return decodedStatements, nil
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
		return common.ToMap(statement)
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

	return common.ToMap(statement)
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
	ed, err := cosign.PemToECDSAKey(raw)
	if err != nil {
		return nil, errors.Wrap(err, "pem to ecdsa")
	}

	return signature.LoadECDSAVerifier(ed, crypto.SHA256)
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

func extractDigest(imgRef string, payload []payload.SimpleContainerImage, log logr.Logger) (string, error) {
	for _, p := range payload {
		if digest := p.Critical.Image.DockerManifestDigest; digest != "" {
			return digest, nil
		} else {
			log.Info("failed to extract image digest from verification response", "image", imgRef, "payload", p)
			return "", fmt.Errorf("unknown image response for " + imgRef)
		}
	}
	return "", fmt.Errorf("digest not found for " + imgRef)
}

func extractIssuer(imgRef string, payload []payload.SimpleContainerImage, log logr.Logger) (string, error) {
	for _, p := range payload {
		if issuer := p.Optional["Issuer"]; issuer != nil {
			return issuer.(string), nil
		} else {
			log.V(3).Info("failed to extract image issuer from verification response", "image", imgRef, "payload", p)
			return "", fmt.Errorf("invalid image response for " + imgRef)
		}
	}
	return "", fmt.Errorf("issuer not found for " + imgRef)
}

func extractSubject(imgRef string, payload []payload.SimpleContainerImage, log logr.Logger) (string, error) {
	for _, p := range payload {
		if subject := p.Optional["Subject"]; subject != nil {
			return subject.(string), nil
		} else {
			log.V(3).Info("failed to extract image subject from verification response", "image", imgRef, "payload", p)
			return "", fmt.Errorf("invalid image response for " + imgRef)
		}
	}
	return "", fmt.Errorf("subject not found for " + imgRef)
}

func checkAnnotations(payload []payload.SimpleContainerImage, annotations map[string]string) error {
	for _, p := range payload {
		for key, val := range annotations {
			if val != p.Optional[key] {
				return fmt.Errorf("annotation value for %s does not match", key)
			}
		}
	}
	return nil
}
