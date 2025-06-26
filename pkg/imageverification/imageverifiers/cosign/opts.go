package cosign

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"
	rekor "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/tuf"
)

func checkOptions(ctx context.Context, att *v1alpha1.Cosign, baseROpts []remote.Option, baseNOpts []name.Option, secretLister imagedataloader.SecretInterface) (*cosign.CheckOpts, error) {
	if err := initializeTuf(ctx, att.TUF); err != nil {
		return nil, err
	}
	cosignRemoteOpts := []ociremote.Option{}

	if att.Source != nil {
		remoteOpts, err := sourceRemoteOpts(ctx, secretLister, att.Source)
		if err != nil {
			return nil, err
		}
		baseROpts = append(baseROpts, remoteOpts...)
		if len(att.Source.Repository) > 0 {
			signatureRepo, err := name.NewRepository(att.Source.Repository)
			if err != nil {
				return nil, fmt.Errorf("failed to parse signature repository %s: %w", att.Source.Repository, err)
			}

			cosignRemoteOpts = append(cosignRemoteOpts, ociremote.WithTargetRepository(signatureRepo))
		}
		if len(att.Source.TagPrefix) != 0 {
			cosignRemoteOpts = append(cosignRemoteOpts, ociremote.WithPrefix(att.Source.TagPrefix))
		}
	}
	cosignRemoteOpts = append(cosignRemoteOpts, ociremote.WithRemoteOptions(baseROpts...), ociremote.WithNameOptions(baseNOpts...))

	opts := &cosign.CheckOpts{
		RegistryClientOpts: cosignRemoteOpts,
	}

	rekorClient, rekorPubKeys, ctlogPubKey, err := getRekor(ctx, att.CTLog)
	if err != nil {
		return nil, fmt.Errorf("getting Rekor public keys:  %w", err)
	}
	opts.RekorClient = rekorClient
	opts.RekorPubKeys = rekorPubKeys
	opts.CTLogPubKeys = ctlogPubKey

	if opts.RekorClient == nil {
		if opts.RekorPubKeys != nil {
			opts.Offline = true
		}
	}

	if att.CTLog != nil {
		opts.IgnoreSCT = att.CTLog.InsecureIgnoreSCT
		opts.IgnoreTlog = att.CTLog.InsecureIgnoreTlog
		if att.CTLog.TSACertChain != "" {
			leaves, intermediates, roots, err := splitCertChain([]byte(att.CTLog.TSACertChain))
			if err != nil {
				return nil, fmt.Errorf("error splitting tsa certificates: %w", err)
			}
			if len(leaves) > 1 {
				return nil, fmt.Errorf("certificate chain must contain at most one TSA certificate")
			}
			if len(leaves) == 1 {
				opts.TSACertificate = leaves[0]
			}
			opts.TSAIntermediateCertificates = intermediates
			opts.TSARootCertificates = roots
		}
	}

	if att.Keyless != nil {
		for _, id := range att.Keyless.Identities {
			opts.Identities = append(opts.Identities,
				cosign.Identity{
					Issuer:        id.Issuer,
					Subject:       id.Subject,
					IssuerRegExp:  id.IssuerRegExp,
					SubjectRegExp: id.SubjectRegExp,
				})
		}
		fulcioRoots, fulcioIntermediates, err := getFulcio(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting Fulcio certs: %w", err)
		}
		opts.RootCerts = fulcioRoots
		opts.IntermediateCerts = fulcioIntermediates
		if att.Keyless.Roots != "" {
			cp, err := certPoolFromBytes([]byte(att.Keyless.Roots))
			if err != nil {
				return nil, fmt.Errorf("failed to load Root certificates: %w", err)
			}
			opts.RootCerts = cp
		}
	} else if att.Key != nil {
		if len(att.Key.Data) > 0 {
			opts.SigVerifier, err = decodePEM([]byte(att.Key.Data), signatureAlgorithmMap[att.Key.HashAlgorithm])
			if err != nil {
				return nil, fmt.Errorf("failed to load public key from PEM: %w", err)
			}
		} else if len(att.Key.KMS) != 0 {
			opts.SigVerifier, err = sigs.PublicKeyFromKeyRefWithHashAlgo(ctx, att.Key.KMS, signatureAlgorithmMap[att.Key.HashAlgorithm])
			if err != nil {
				return nil, fmt.Errorf("failed to load public key from %s: %w", att.Key.KMS, err)
			}
		}
	} else if att.Certificate != nil {
		if att.Certificate.Certificate != nil && att.Certificate.Certificate.Value != "" {
			// load cert and optionally a cert chain as a verifier
			cert, err := certFromBytes([]byte(att.Certificate.Certificate.Value))
			if err != nil {
				return nil, fmt.Errorf("failed to load certificate from %s: %w", att.Certificate.Certificate, err)
			}

			if att.Certificate.CertificateChain != nil && att.Certificate.CertificateChain.Value == "" {
				opts.SigVerifier, err = signature.LoadVerifier(cert.PublicKey, signatureAlgorithmMap[att.Key.HashAlgorithm])
				if err != nil {
					return nil, fmt.Errorf("failed to load signature from certificate: %w", err)
				}
			} else {
				// Verify certificate with chain
				chain, err := certChainFromBytes([]byte(att.Certificate.CertificateChain.Value))
				if err != nil {
					return nil, fmt.Errorf("failed to load load certificate chain: %w", err)
				}
				opts.SigVerifier, err = cosign.ValidateAndUnpackCertWithChain(cert, chain, opts)
				if err != nil {
					return nil, fmt.Errorf("failed to load validate certificate chain: %w", err)
				}
			}
		}
		if att.Certificate.CertificateChain != nil && att.Certificate.CertificateChain.Value != "" {
			// load cert chain as roots
			cp, err := certPoolFromBytes([]byte(att.Certificate.CertificateChain.Value))
			if err != nil {
				return nil, fmt.Errorf("failed to load certificates: %w", err)
			}
			opts.RootCerts = cp
		}
	}
	return opts, nil
}

func initializeTuf(ctx context.Context, t *v1alpha1.TUF) error {
	if t != nil {
		var root []byte
		var err error
		if t.Root.Path != "" {
			root, err = blob.LoadFileOrURL(t.Root.Path)
			if err != nil {
				return fmt.Errorf("Failed to read alternate TUF root file %v : %w", t, err)
			}
		} else if t.Root.Data != "" {
			root, err = base64.StdEncoding.DecodeString(t.Root.Data)
			if err != nil {
				return fmt.Errorf("Failed to base64 decode TUF root  %v : %w", t, err)
			}
		}

		if err := tuf.Initialize(ctx, t.Mirror, root); err != nil {
			return fmt.Errorf("Failed to initialize TUF client from %v : %w", t, err)
		}
	} else {
		if err := tuf.Initialize(ctx, tuf.DefaultRemoteRoot, nil); err != nil {
			return fmt.Errorf("Failed to initialize TUF client from %v : %w", t, err)
		}
	}
	return nil
}

func sourceRemoteOpts(ctx context.Context, secretLister imagedataloader.SecretInterface, src *v1alpha1.Source) ([]remote.Option, error) {
	opts := make([]remote.Option, 0)
	if len(src.SignaturePullSecrets) > 0 {
		signaturePullSecrets := make([]string, 0, len(src.SignaturePullSecrets))
		for _, s := range src.SignaturePullSecrets {
			signaturePullSecrets = append(signaturePullSecrets, s.Name)
		}
		kc, err := imagedataloader.NewAutoRefreshSecretsKeychain(secretLister, signaturePullSecrets...)
		if err != nil {
			return nil, err
		}
		opts = append(opts, remote.WithAuthFromKeychain(kc))
	}
	return opts, nil
}

func getRekor(ctx context.Context, ctlog *v1alpha1.CTLog) (*client.Rekor, *cosign.TrustedTransparencyLogPubKeys, *cosign.TrustedTransparencyLogPubKeys, error) {
	// In keyless, if no TrustRoot was defined and CTLog is nil, then default to rekor pub keys as done in cosign
	if ctlog == nil {
		rekorPubKeys, err := cosign.GetRekorPubs(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("getting Rekor public keys: %w", err)
		}
		ctlogPubKey, err := cosign.GetCTLogPubs(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("getting Rekor public keys: %w", err)
		}
		return nil, rekorPubKeys, ctlogPubKey, nil
	}

	if len(ctlog.URL) == 0 {
		return nil, nil, nil, fmt.Errorf("rekor URL must be provided")
	}
	rekorClient, err := rekor.GetRekorClient(ctlog.URL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error creating Rekor client: %w", err)
	}

	var rekorPubKey *cosign.TrustedTransparencyLogPubKeys
	var ctlogPubKey *cosign.TrustedTransparencyLogPubKeys
	if ctlog.RekorPubKey == "" {
		if rekorPubKey, err = cosign.GetRekorPubs(ctx); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get rekor public keys: %w", err)
		}
	} else {
		key := cosign.NewTrustedTransparencyLogPubKeys()
		if err := key.AddTransparencyLogPubKey([]byte(ctlog.RekorPubKey), tuf.Active); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse rekor public keys: %w", err)
		}
		rekorPubKey = &key
	}

	if ctlog.CTLogPubKey == "" {
		if ctlogPubKey, err = cosign.GetCTLogPubs(ctx); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get ctlog public keys: %w", err)
		}
	} else {
		key := cosign.NewTrustedTransparencyLogPubKeys()
		if err := key.AddTransparencyLogPubKey([]byte(ctlog.CTLogPubKey), tuf.Active); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse ctlog public keys: %w", err)
		}
		ctlogPubKey = &key
	}

	return rekorClient, rekorPubKey, ctlogPubKey, nil
}

func getFulcio(ctx context.Context) (*x509.CertPool, *x509.CertPool, error) {
	roots, err := fulcioroots.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Fulcio roots: %w", err)
	}
	intermediates, err := fulcioroots.GetIntermediates()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch Fulcio intermediates: %w", err)
	}
	return roots, intermediates, nil
}
