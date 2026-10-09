package manifest

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/sigstoreguard"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v3/pkg/oci"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/k8smanifest"
	manifestutil "github.com/sigstore/k8s-manifest-sigstore/pkg/util"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/util/kubeutil"
	"github.com/sigstore/k8s-manifest-sigstore/pkg/util/sigtypes/pgp"
	sigx509 "github.com/sigstore/k8s-manifest-sigstore/pkg/util/sigtypes/x509"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func verifySignatures(ctx context.Context, obj unstructured.Unstructured, signatureRef string, vo *k8smanifest.VerifyResourceOption) (bool, string, error) {
	keys := manifestutil.SplitCommaSeparatedString(vo.KeyPath)
	if len(keys) == 0 {
		keys = []string{""}
	}
	image := signatureRef != "" && signatureRef != k8smanifest.SigRefEmbeddedInAnnotation && !strings.HasPrefix(signatureRef, kubeutil.InClusterObjectPrefix)
	var sets []map[string]string
	if !image {
		var err error
		sets, err = signatureSets(obj, signatureRef, vo)
		if err != nil {
			return false, "", err
		}
		if len(sets) == 0 {
			return false, "", k8smanifest.NewSignatureNotFoundError(nil)
		}
	}
	var failures []error
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return false, "", err
		}
		if image {
			signer, err := verifyImage(ctx, signatureRef, key, vo)
			if err == nil && matchesSigner(vo, key, signer) {
				return true, signer, nil
			}
			if err == nil {
				err = fmt.Errorf("signature signer %q does not match the configured signer", signer)
			}
			failures = append(failures, err)
			continue
		}
		for _, set := range sets {
			signer, err := verifyBlob(ctx, set, key, vo)
			if err == nil && matchesSigner(vo, key, signer) {
				return true, signer, nil
			}
			if err == nil {
				err = fmt.Errorf("signature signer %q does not match the configured signer", signer)
			}
			failures = append(failures, err)
		}
	}
	return false, "", fmt.Errorf("signature verification failed: %w", errors.Join(failures...))
}

func matchesSigner(vo *k8smanifest.VerifyResourceOption, key, signer string) bool {
	// The legacy verifier applies its inner exact identity check only to
	// keyless identities. Keyed identities still use the final SignerList.Match.
	return key != "" || len(vo.Signers) == 0 || slices.Contains(vo.Signers, "") || slices.Contains(vo.Signers, signer)
}

func verifyImage(ctx context.Context, image, key string, vo *k8smanifest.VerifyResourceOption) (string, error) {
	opts, nameOpts := registryclient.GlobalOptsOrDefault(ctx)
	if vo.AllowInsecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return "", err
	}
	verifier, err := loadVerifier(ctx, key)
	if err != nil {
		return "", err
	}
	defer closeVerifier(verifier)
	co, err := checkOptions(ctx, verifier, vo, false, true)
	if err != nil {
		return "", err
	}
	co.RegistryClientOpts = []ociremote.Option{ociremote.WithRemoteOptions(opts...), ociremote.WithNameOptions(nameOpts...)}
	co.ClaimVerifier = cosign.SimpleClaimVerifier
	return verifyImageSignatures(ctx, ref, co, key, vo)
}

func verifyImageSignatures(ctx context.Context, ref name.Reference, co *cosign.CheckOpts, key string, vo *k8smanifest.VerifyResourceOption) (string, error) {
	signatures, _, err := cosign.VerifyImageSignatures(ctx, ref, co)
	if err != nil {
		return "", err
	}
	if len(signatures) == 0 {
		return "", fmt.Errorf("no verified signatures in manifest image")
	}
	var firstSigner string
	haveSigner := false
	for _, sig := range signatures {
		cert, err := sig.Cert()
		if err != nil {
			continue
		}
		signer := ""
		if cert != nil {
			signer = manifestutil.GetNameInfoFromCert(cert)
		}
		if !haveSigner {
			firstSigner, haveSigner = signer, true
		}
		if matchesSigner(vo, key, signer) && vo.Signers.Match(signer) {
			return signer, nil
		}
	}
	// Preserve the existing mismatch result when none of the verified signers
	// is allowed. The caller's identity checks will reject this first signer.
	return firstSigner, nil
}

func verifyBlob(ctx context.Context, set map[string]string, key string, vo *k8smanifest.VerifyResourceOption) (string, error) {
	verifier, err := loadVerifier(ctx, key)
	if err != nil {
		if errors.Is(err, egress.ErrAddressBlocked) || ctx.Err() != nil {
			return "", err
		}
		// PGP and X.509 legacy references only read files/env/Kubernetes secrets.
		// Never call upstream's signature-type detector: its Cosign probe fetches
		// arbitrary URLs with an unguarded HTTP client.
		if _, pgpErr := pgp.LoadPublicKey(key); pgpErr == nil {
			ok, signer, _, verifyErr := pgp.VerifyBlob([]byte(set["message"]), []byte(set["signature"]), &key)
			if verifyErr != nil {
				return "", verifyErr
			}
			if !ok {
				return "", fmt.Errorf("PGP signature verification failed")
			}
			return signer, nil
		}
		if _, certErr := sigx509.LoadCertificate(key); certErr == nil {
			ok, signer, _, verifyErr := sigx509.VerifyBlob([]byte(set["message"]), []byte(set["signature"]), []byte(set["certificate"]), &key)
			if verifyErr != nil {
				return "", verifyErr
			}
			if !ok {
				return "", fmt.Errorf("X.509 signature verification failed")
			}
			return signer, nil
		}
		return "", err
	}
	defer closeVerifier(verifier)
	message, err := decodeAnnotation(set["message"])
	if err != nil {
		return "", err
	}
	cert, err := decodeAnnotation(set["certificate"])
	if err != nil {
		return "", err
	}
	rawBundle, err := decodeAnnotation(set["bundle"])
	if err != nil {
		return "", err
	}
	ignoreTlog := len(cert) == 0 && len(rawBundle) == 0
	co, err := checkOptions(ctx, verifier, vo, ignoreTlog, len(rawBundle) > 0)
	if err != nil {
		return "", err
	}
	var options []static.Option
	if len(cert) == 0 && vo.Certificate != "" {
		cert, err = certificatePEM(ctx, vo.Certificate)
		if err != nil {
			return "", err
		}
	}
	var chain []byte
	if vo.CertificateChain != "" {
		chain, err = certificatePEM(ctx, vo.CertificateChain)
		if err != nil {
			return "", err
		}
	}
	if len(cert) > 0 {
		options = append(options, static.WithCertChain(cert, chain))
	}
	if len(rawBundle) > 0 {
		var entry bundle.RekorBundle
		if err := json.Unmarshal(rawBundle, &entry); err != nil {
			return "", fmt.Errorf("decoding Rekor bundle: %w", err)
		}
		options = append(options, static.WithBundle(&entry))
	}
	sig, err := static.NewSignature(message, set["signature"], options...)
	if err != nil {
		return "", err
	}
	if _, err := cosign.VerifyBlobSignature(ctx, sig, co); err != nil {
		return "", err
	}
	return blobSigner(sig)
}

func loadVerifier(ctx context.Context, key string) (signature.Verifier, error) {
	if key == "" {
		return nil, nil
	}
	return sigstoreguard.PublicKeyFromKeyRefWithHashAlgo(ctx, key, crypto.SHA256)
}

func closeVerifier(verifier signature.Verifier) {
	if closer, ok := verifier.(interface{ Close() }); ok {
		closer.Close()
	}
}

func checkOptions(ctx context.Context, verifier signature.Verifier, vo *k8smanifest.VerifyResourceOption, ignoreTlog, ignoreSCT bool) (*cosign.CheckOpts, error) {
	co := &cosign.CheckOpts{SigVerifier: verifier, IgnoreTlog: ignoreTlog, IgnoreSCT: ignoreSCT, RootCerts: vo.RootCerts, Identities: []cosign.Identity{{Issuer: vo.OIDCIssuer}}}
	var err error
	if !ignoreTlog {
		co.RekorPubKeys, err = sigstoreguard.RekorPublicKeys(ctx)
		if err != nil {
			return nil, err
		}
		endpoint := vo.RekorURL
		if endpoint == "" {
			endpoint = os.Getenv("REKOR_SERVER")
		}
		if endpoint == "" {
			endpoint = "https://rekor.sigstore.dev"
		}
		co.RekorClient, err = sigstoreguard.NewRekorClient(endpoint)
		if err != nil {
			return nil, err
		}
	}
	if verifier == nil {
		// A chain supplied by the policy is an explicit trust anchor. A chain
		// carried only on an untrusted signature never becomes a trusted root.
		if vo.CertificateChain != "" {
			chain, err := loadCertificates(ctx, vo.CertificateChain)
			if err != nil {
				return nil, err
			}
			if len(chain) == 0 {
				return nil, fmt.Errorf("certificate chain contains no certificates")
			}
			if co.RootCerts == nil {
				co.RootCerts = x509.NewCertPool()
			} else {
				co.RootCerts = co.RootCerts.Clone()
			}
			co.RootCerts.AddCert(chain[len(chain)-1])
			co.IntermediateCerts = x509.NewCertPool()
			for _, cert := range chain[:len(chain)-1] {
				co.IntermediateCerts.AddCert(cert)
			}
		}
		if co.RootCerts == nil && vo.Certificate == "" {
			co.RootCerts, co.IntermediateCerts, err = sigstoreguard.FulcioRootsWithContext(ctx)
			if err != nil {
				return nil, err
			}
		}
		if !ignoreSCT && !ignoreTlog {
			co.CTLogPubKeys, err = sigstoreguard.CTLogPublicKeys(ctx)
			if err != nil {
				return nil, err
			}
		}
		if vo.Certificate != "" {
			certs, err := loadCertificates(ctx, vo.Certificate)
			if err != nil {
				return nil, err
			}
			if len(certs) == 0 {
				return nil, fmt.Errorf("certificate reference contains no certificates")
			}
			if vo.CertificateChain == "" {
				if err := cosign.CheckCertificatePolicy(certs[0], co); err != nil {
					return nil, err
				}
				co.SigVerifier, err = signature.LoadVerifier(certs[0].PublicKey, crypto.SHA256)
				if err != nil {
					return nil, err
				}
			} else {
				chain, err := loadCertificates(ctx, vo.CertificateChain)
				if err != nil {
					return nil, err
				}
				co.SigVerifier, err = cosign.ValidateAndUnpackCertWithChain(certs[0], chain, co)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return co, nil
}

func decodeAnnotation(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	compressed, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decoding manifest signature annotation: %w", err)
	}
	return manifestutil.GzipDecompress(compressed), nil
}

func blobSigner(sig oci.Signature) (string, error) {
	cert, err := sig.Cert()
	if err != nil {
		return "", err
	}
	if cert == nil {
		return "", nil
	}
	return sigx509.GetNameInfoFromX509Cert(cert), nil
}

// Kubernetes references use the operator API client; HTTP references always use
// the egress guard. Keep file/env references compatible with legacy manifests.
func loadCertificates(ctx context.Context, ref string) ([]*x509.Certificate, error) {
	if strings.HasPrefix(ref, kubeutil.InClusterObjectPrefix) {
		return sigx509.LoadCertificateChain(ref)
	}
	data, err := sigstoreguard.LoadFileOrURL(ctx, ref)
	if err != nil {
		return nil, err
	}
	return cryptoutils.LoadCertificatesFromPEM(bytes.NewReader(data))
}

func certificatePEM(ctx context.Context, ref string) ([]byte, error) {
	certs, err := loadCertificates(ctx, ref)
	if err != nil {
		return nil, err
	}
	return cryptoutils.MarshalCertificatesToPEM(certs)
}
