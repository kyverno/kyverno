package cosign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/sigstore/cosign/v2/pkg/oci"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
)

var signatureAlgorithmMap = map[string]crypto.Hash{
	"":       crypto.SHA256,
	"sha224": crypto.SHA224,
	"sha256": crypto.SHA256,
	"sha384": crypto.SHA384,
	"sha512": crypto.SHA512,
}

func certPoolFromBytes(roots []byte) (*x509.CertPool, error) {
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(roots) {
		return nil, fmt.Errorf("error creating root cert pool")
	}

	return cp, nil
}

func certFromBytes(pem []byte) (*x509.Certificate, error) {
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

func certChainFromBytes(pem []byte) ([]*x509.Certificate, error) {
	return cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(pem))
}

func splitCertChain(pem []byte) (leaves, intermediates, roots []*x509.Certificate, err error) {
	certs, err := cryptoutils.UnmarshalCertificatesFromPEM(pem)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, cert := range certs {
		if !cert.IsCA {
			leaves = append(leaves, cert)
		} else {
			// root certificates are self-signed
			if bytes.Equal(cert.RawSubject, cert.RawIssuer) {
				roots = append(roots, cert)
			} else {
				intermediates = append(intermediates, cert)
			}
		}
	}

	return leaves, intermediates, roots, nil
}

func decodePEM(raw []byte, signatureAlgorithm crypto.Hash) (signature.Verifier, error) {
	// PEM encoded file.
	pubKey, err := cryptoutils.UnmarshalPEMToPublicKey(raw)
	if err != nil {
		return nil, fmt.Errorf("pem to public key: %w", err)
	}

	return signature.LoadVerifier(pubKey, signatureAlgorithm)
}

func checkSignatureAnnotations(sig oci.Signature, annotations map[string]string) error {
	sigAnnotations, err := sig.Annotations()
	if err != nil {
		return fmt.Errorf("failed to fetch annotation from signature")
	}
	for key, val := range annotations {
		if val != sigAnnotations[key] {
			return fmt.Errorf("annotations mismatch: %s does not match expected value %s for key %s",
				sigAnnotations[key], val, key)
		}
	}
	return nil
}
