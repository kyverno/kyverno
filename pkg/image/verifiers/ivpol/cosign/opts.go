package cosign

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/sigstoretuf"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/sigstore/cosign/v3/pkg/blob"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/v3/pkg/signature"
	rekor "github.com/sigstore/rekor/pkg/client"
	"github.com/sigstore/rekor/pkg/generated/client"
	"github.com/sigstore/sigstore-go/pkg/root"
	sigstoreTuf "github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/tuf"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var (
	tufInitializeFn = func(ctx context.Context, mirror string, rootBytes []byte) error {
		return tuf.Initialize(ctx, mirror, rootBytes)
	}
	getRekorPubsFn = func(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
		return cosign.GetRekorPubs(ctx)
	}
	getCTLogPubsFn = func(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
		return cosign.GetCTLogPubs(ctx)
	}
	getFulcioRootsFn    = func() (*x509.CertPool, error) { return fulcioroots.Get() }
	getFulcioIntermedFn = func() (*x509.CertPool, error) { return fulcioroots.GetIntermediates() }
	getTrustedRootFromTUFFn = func(ctx context.Context, t *v1beta1.TUF) (*root.TrustedRoot, error) {
		return getTrustedRootFromTUF(ctx, t)
	}
)

// maxIntermediateCerts limits the number of intermediate certificates accepted
// from user-provided certificate chains to mitigate CVE-2026-32280 (DoS via
// unbounded work in crypto/x509 certificate chain building).
const maxIntermediateCerts = 10

// maxTrustedRootJSONSize limits the size of an inline trustedRoot JSON value
// (att.TrustedRoot.Value) accepted for parsing. This value can be
// user-controlled (e.g. sourced via a CEL expression reading a ConfigMap), so
// an unbounded size could cause excessive memory/CPU usage during policy
// evaluation. 1 MiB is generous relative to real-world trusted root
// documents (GitHub's is ~26 KB).
const maxTrustedRootJSONSize = 1 << 20 // 1 MiB

// pemCertBlockHeader is the PEM block header used to count certificate blocks
// cheaply before full ASN.1 parsing.
var pemCertBlockHeader = []byte("-----BEGIN CERTIFICATE-----")

// countPEMCertBlocks returns the number of CERTIFICATE PEM blocks in the input
// using a cheap byte scan, so we can reject oversized chains before doing the
// expensive PEM/ASN.1 parsing work.
func countPEMCertBlocks(pem []byte) int {
	return bytes.Count(pem, pemCertBlockHeader)
}

func checkOptions(ctx context.Context, att *v1beta1.Cosign, baseROpts []remote.Option, baseNOpts []name.Option, secretLister corev1listers.SecretLister) (*cosign.CheckOpts, error) {
	// Key/certificate verification with the transparency log ignored needs no
	// Sigstore infrastructure (TUF, Rekor, CTLog), mirroring cosign.
	ignoreTlog := att.CTLog != nil && att.CTLog.InsecureIgnoreTlog
	keyOrCert := att.Keyless == nil && (att.Key != nil || att.Certificate != nil)
	skipSigstoreInfra := keyOrCert && ignoreTlog
	cosignRemoteOpts := []ociremote.Option{}

	if att.Source != nil {
		remoteOpts, err := sourceRemoteOpts(secretLister, att.Source)
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

	var err error
	var trust *sigstoreTrustMaterial
	opts := &cosign.CheckOpts{
		RegistryClientOpts: cosignRemoteOpts,
	}

	if !skipSigstoreInfra {
		// initTUFAndFetch initializes the TUF singleton and reads all
		// TUF-derived trust material (Rekor/CTLog pubkeys, trusted root,
		// Fulcio roots) in a single mutex acquisition, so no concurrent
		// goroutine can reinitialize the singleton with a different mirror
		// between the init step and the reads.
		trust, err = initTUFAndFetch(ctx, att.TUF)
		if err != nil {
			return nil, err
		}

		rekorClient, rekorPubKeys, ctlogPubKey, err := getRekor(ctx, att.CTLog, trust.rekorPubKeys, trust.ctlogPubKeys)
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

		trustedMaterial, err := resolveTrustedMaterial(att, trust.trustedRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve trusted material: %w", err)
		}
		opts.TrustedMaterial = trustedMaterial
	}

	if att.CTLog != nil {
		opts.IgnoreSCT = att.CTLog.InsecureIgnoreSCT
		opts.IgnoreTlog = att.CTLog.InsecureIgnoreTlog
		if att.CTLog.TSACertChain != "" {
			// Cheap pre-check on the raw PEM to reject oversized chains
			// before expensive ASN.1 parsing (CVE-2026-32280).
			if n := countPEMCertBlocks([]byte(att.CTLog.TSACertChain)); n > maxIntermediateCerts+2 {
				return nil, fmt.Errorf("TSA certificate chain contains too many certificates (%d), maximum allowed is %d", n, maxIntermediateCerts+2)
			}
			leaves, intermediates, roots, err := splitCertChain([]byte(att.CTLog.TSACertChain))
			if err != nil {
				return nil, fmt.Errorf("error splitting tsa certificates: %w", err)
			}
			if len(leaves) > 1 {
				return nil, fmt.Errorf("certificate chain must contain at most one TSA certificate")
			}
			if len(intermediates) > maxIntermediateCerts {
				return nil, fmt.Errorf("TSA certificate chain contains too many intermediate certificates (%d), maximum allowed is %d", len(intermediates), maxIntermediateCerts)
			}
			if len(leaves) == 1 {
				opts.TSACertificate = leaves[0]
			}
			opts.TSAIntermediateCertificates = intermediates
			opts.TSARootCertificates = roots
			opts.UseSignedTimestamps = true
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
		// trust is always non-nil when att.Keyless != nil because
		// skipSigstoreInfra requires keyOrCert=true (att.Keyless==nil).
		opts.RootCerts = trust.fulcioRoots
		opts.IntermediateCerts = trust.fulcioIntermediates
		if att.Keyless.Roots != "" {
			cp, err := certPoolFromBytes([]byte(att.Keyless.Roots))
			if err != nil {
				return nil, fmt.Errorf("failed to load Root certificates: %w", err)
			}
			opts.RootCerts = cp
		}

		return opts, nil
	}

	if att.Key != nil {
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
		return opts, nil
	}

	if att.Certificate != nil {
		if att.Certificate.Certificate != nil && att.Certificate.Certificate.Value != "" {
			// load cert and optionally a cert chain as a verifier
			cert, err := certFromBytes([]byte(att.Certificate.Certificate.Value))
			if err != nil {
				return nil, fmt.Errorf("failed to load certificate from %s: %w", att.Certificate.Certificate, err)
			}

			if att.Certificate.CertificateChain == nil || att.Certificate.CertificateChain.Value == "" {
				opts.SigVerifier, err = signature.LoadVerifier(cert.PublicKey, signatureAlgorithmMap[""])
				if err != nil {
					return nil, fmt.Errorf("failed to load signature from certificate: %w", err)
				}
			} else {
				// Verify certificate with chain.
				// Cheap pre-check on the raw PEM to reject oversized chains
				// before expensive ASN.1 parsing (CVE-2026-32280).
				if n := countPEMCertBlocks([]byte(att.Certificate.CertificateChain.Value)); n > maxIntermediateCerts+1 {
					return nil, fmt.Errorf("certificate chain too long (%d), maximum allowed is %d", n, maxIntermediateCerts+1)
				}
				chain, err := certChainFromBytes([]byte(att.Certificate.CertificateChain.Value))
				if err != nil {
					return nil, fmt.Errorf("failed to load certificate chain: %w", err)
				}
				if len(chain) > maxIntermediateCerts+1 {
					return nil, fmt.Errorf("certificate chain too long (%d), maximum allowed is %d", len(chain), maxIntermediateCerts+1)
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

		return opts, nil
	}
	return nil, fmt.Errorf("cosign verifier needs to have one key, keyless or certificate fields set")
}

// sigstoreTrustMaterial holds all TUF-derived trust material fetched by
// initTUFAndFetch in a single locked operation.
type sigstoreTrustMaterial struct {
	rekorPubKeys        *cosign.TrustedTransparencyLogPubKeys
	ctlogPubKeys        *cosign.TrustedTransparencyLogPubKeys
	trustedRoot         *root.TrustedRoot
	fulcioRoots         *x509.CertPool
	fulcioIntermediates *x509.CertPool
}

// initTUFAndFetch pre-reads any file-based TUF root (pure I/O), then
// atomically initializes the TUF singleton and fetches all TUF-derived
// trust material within a single sigstoretuf.WithLock acquisition.
// This prevents a concurrent goroutine from reinitializing the singleton
// with a different mirror between the initialization and the reads.
//
// Uses the sigstore-go TUF client (go-tuf/v2) for trusted_root.json to
// support TAP-15 succinct_roles (hash-bin delegations), and falls back
// to trusted_root.json when individual TUF targets are missing.
func initTUFAndFetch(ctx context.Context, t *v1beta1.TUF) (*sigstoreTrustMaterial, error) {
	// Step 1: read optional root bytes — pure I/O, no TUF lock needed.
	var rootBytes []byte
	mirror := tuf.DefaultRemoteRoot
	if t != nil {
		var err error
		if t.Root.Path != "" {
			rootBytes, err = blob.LoadFileOrURL(t.Root.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to read alternate TUF root file %q: %w", t.Root.Path, err)
			}
		} else if t.Root.Data != "" {
			rootBytes, err = base64.StdEncoding.DecodeString(t.Root.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to base64-decode inline TUF root data: %w", err)
			}
		}
		if t.Mirror != "" {
			mirror = t.Mirror
		}
	}

	// Step 2: hold the process-wide TUF mutex for init + all reads so
	// that no other goroutine can reinitialize the singleton between them.
	// Note: fn must call sigstore/TUF functions directly (not through
	// sigstoretuf wrappers) to avoid deadlocking on the same mutex.
	var m sigstoreTrustMaterial
	err := sigstoretuf.WithLock(func() error {
		if err := tufInitializeFn(ctx, mirror, rootBytes); err != nil {
			return fmt.Errorf("failed to initialize TUF client (mirror=%q): %w", mirror, err)
		}

		// Fetch trusted_root.json via sigstore-go TUF client (go-tuf/v2)
		// instead of tuf.NewFromEnv (go-tuf v0.7.0) to support TAP-15
		// succinct_roles (hash-bin delegations).
		tr, trustedRootErr := getTrustedRootFromTUFFn(ctx, t)
		if trustedRootErr != nil {
			return fmt.Errorf("getting trusted root: %w", trustedRootErr)
		}
		m.trustedRoot = tr

		// Fetch Rekor public keys — may fail for private TUF mirrors
		// that don't serve individual targets; fall back to trusted root.
		var err error
		m.rekorPubKeys, err = getRekorPubsFn(ctx)
		if err != nil {
			m.rekorPubKeys, err = rekorPubsFromTrustedRoot(m.trustedRoot)
			if err != nil {
				return fmt.Errorf("getting Rekor public keys: %w", err)
			}
		}

		// Fetch CTLog public keys — same caveat as Rekor.
		m.ctlogPubKeys, err = getCTLogPubsFn(ctx)
		if err != nil {
			m.ctlogPubKeys, err = ctLogPubsFromTrustedRoot(m.trustedRoot)
			if err != nil {
				return fmt.Errorf("getting CTLog public keys: %w", err)
			}
		}

		// Fetch Fulcio material — individual targets may be absent.
		rootsErr := error(nil)
		intermedErr := error(nil)
		m.fulcioRoots, rootsErr = getFulcioRootsFn()
		m.fulcioIntermediates, intermedErr = getFulcioIntermedFn()
		if rootsErr != nil || intermedErr != nil {
			fRoots, fIntermediates, fErr := fulcioRootsFromTrustedRoot(m.trustedRoot)
			if fErr != nil {
				return fmt.Errorf("failed to fetch Fulcio material and trusted root fallback also failed: %w", fErr)
			}
			if rootsErr != nil {
				m.fulcioRoots = fRoots
			}
			if intermedErr != nil {
				m.fulcioIntermediates = fIntermediates
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func sourceRemoteOpts(secretLister corev1listers.SecretLister, src *v1beta1.Source) ([]remote.Option, error) {
	opts := make([]remote.Option, 0)
	if len(src.SignaturePullSecrets) > 0 {
		signaturePullSecrets := make([]string, 0, len(src.SignaturePullSecrets))
		for _, s := range src.SignaturePullSecrets {
			signaturePullSecrets = append(signaturePullSecrets, s.Name)
		}
		kc := regcreds.NewSecretsKeychain(secretLister, config.KyvernoNamespace(), signaturePullSecrets...)
		opts = append(opts, remote.WithAuthFromKeychain(kc))
	}
	return opts, nil
}

// getRekor resolves the Rekor client and the Rekor/CTLog transparency log
// public keys used to verify signed entry timestamps (SET) and signed
// certificate timestamps (SCT).
//
// When ctlog.InsecureIgnoreTlog is set, no Rekor URL or client is required:
// the transparency log verification will be skipped entirely by cosign, so
// building a Rekor client (which requires a URL) is unnecessary and should
// not fail policy evaluation. Similarly, when ctlog.InsecureIgnoreSCT is set,
// fetching CT log public keys is skipped.
//
// defaultRekorPubKeys/defaultCtlogPubKeys are the Rekor/CTLog public keys
// already fetched atomically alongside TUF initialization by
// initTUFAndFetch; they are reused here (when the policy does not override
// them explicitly) to avoid an extra TUF lock acquisition.
func getRekor(_ context.Context, ctlog *v1beta1.CTLog, defaultRekorPubKeys, defaultCtlogPubKeys *cosign.TrustedTransparencyLogPubKeys) (*client.Rekor, *cosign.TrustedTransparencyLogPubKeys, *cosign.TrustedTransparencyLogPubKeys, error) {
	// When CTLog is nil, default to the Rekor/CTLog pubkeys already fetched
	// atomically with initTUFAndFetch.
	if ctlog == nil {
		return nil, defaultRekorPubKeys, defaultCtlogPubKeys, nil
	}

	var (
		rekorClient *client.Rekor
		rekorPubKey *cosign.TrustedTransparencyLogPubKeys
		ctlogPubKey *cosign.TrustedTransparencyLogPubKeys
		err         error
	)

	if !ctlog.InsecureIgnoreTlog {
		if len(ctlog.URL) == 0 {
			return nil, nil, nil, fmt.Errorf("rekor URL must be provided")
		}
		rekorClient, err = rekor.GetRekorClient(ctlog.URL)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error creating Rekor client: %w", err)
		}
		if ctlog.RekorPubKey == "" {
			// Reuse pre-fetched defaults to avoid an extra TUF lock acquisition.
			rekorPubKey = defaultRekorPubKeys
		} else {
			key := cosign.NewTrustedTransparencyLogPubKeys()
			if err := key.AddTransparencyLogPubKey([]byte(ctlog.RekorPubKey), tuf.Active); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse rekor public keys: %w", err)
			}
			rekorPubKey = &key
		}
	}
	if !ctlog.InsecureIgnoreSCT {
		if ctlog.CTLogPubKey == "" {
			// Reuse pre-fetched defaults to avoid an extra TUF lock acquisition.
			ctlogPubKey = defaultCtlogPubKeys
		} else {
			key := cosign.NewTrustedTransparencyLogPubKeys()
			if err := key.AddTransparencyLogPubKey([]byte(ctlog.CTLogPubKey), tuf.Active); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse ctlog public keys: %w", err)
			}
			ctlogPubKey = &key
		}
	}

	return rekorClient, rekorPubKey, ctlogPubKey, nil
}

// getTrustedRootFromTUF fetches trusted_root.json from the TUF repository
// using the sigstore-go TUF client (go-tuf/v2) which supports TAP-15
// succinct_roles (hash-bin delegations). This is the core fix for private
// Sigstore TUF mirrors (e.g. RSTUF) that serve targets via hash-bin
// delegations.
func getTrustedRootFromTUF(ctx context.Context, t *v1beta1.TUF) (*root.TrustedRoot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts := sigstoreTuf.DefaultOptions().WithContext(ctx)
	if t != nil {
		if t.Mirror != "" {
			opts.RepositoryBaseURL = t.Mirror
		}
		if t.Root.Path != "" {
			rootBytes, err := blob.LoadFileOrURL(t.Root.Path)
			if err != nil {
				return nil, fmt.Errorf("loading TUF root: %w", err)
			}
			opts.Root = rootBytes
		} else if t.Root.Data != "" {
			rootBytes, err := base64.StdEncoding.DecodeString(t.Root.Data)
			if err != nil {
				return nil, fmt.Errorf("decoding TUF root: %w", err)
			}
			opts.Root = rootBytes
		}
	}
	client, err := sigstoreTuf.New(opts)
	if err != nil {
		return nil, fmt.Errorf("creating sigstore-go TUF client: %w", err)
	}
	targetBytes, err := client.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json from TUF: %w", err)
	}
	trustedRoot, err := root.NewTrustedRootFromJSON(targetBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}
	return trustedRoot, nil
}

// rekorPubsFromTrustedRoot extracts Rekor transparency log public keys from
// a TrustedRoot, used as a fallback when individual TUF targets (rekor.pub)
// are unavailable.
func rekorPubsFromTrustedRoot(tr *root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if tr == nil {
		return nil, fmt.Errorf("trusted root not available")
	}
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	added := 0
	var lastErr error
	for _, tlog := range tr.RekorLogs() {
		pemBytes, err := publicKeyToPEM(tlog.PublicKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to encode Rekor public key: %w", err)
			continue
		}
		if err := keys.AddTransparencyLogPubKey(pemBytes, logKeyStatus(tlog.ValidityPeriodStart, tlog.ValidityPeriodEnd)); err != nil {
			lastErr = fmt.Errorf("failed to add Rekor public key: %w", err)
			continue
		}
		added++
	}
	if added == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no Rekor public keys found in trusted root")
	}
	return &keys, nil
}

// ctLogPubsFromTrustedRoot extracts CT log public keys from a TrustedRoot,
// used as a fallback when individual TUF targets (ctfe.pub) are unavailable.
func ctLogPubsFromTrustedRoot(tr *root.TrustedRoot) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if tr == nil {
		return nil, fmt.Errorf("trusted root not available")
	}
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	added := 0
	var lastErr error
	for _, tlog := range tr.CTLogs() {
		pemBytes, err := publicKeyToPEM(tlog.PublicKey)
		if err != nil {
			lastErr = fmt.Errorf("failed to encode CTLog public key: %w", err)
			continue
		}
		if err := keys.AddTransparencyLogPubKey(pemBytes, logKeyStatus(tlog.ValidityPeriodStart, tlog.ValidityPeriodEnd)); err != nil {
			lastErr = fmt.Errorf("failed to add CTLog public key: %w", err)
			continue
		}
		added++
	}
	if added == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("no CTLog public keys found in trusted root")
	}
	return &keys, nil
}

// fulcioRootsFromTrustedRoot extracts Fulcio root and intermediate
// certificates from a TrustedRoot, used as a fallback when individual
// TUF targets (fulcio_v1.crt.pem) are unavailable.
func fulcioRootsFromTrustedRoot(tr *root.TrustedRoot) (*x509.CertPool, *x509.CertPool, error) {
	if tr == nil {
		return nil, nil, fmt.Errorf("trusted root not available")
	}
	cas := tr.FulcioCertificateAuthorities()
	if len(cas) == 0 {
		return nil, nil, fmt.Errorf("no certificate authority in trusted root")
	}
	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()
	rootsAdded := 0
	for _, ca := range cas {
		fca, ok := ca.(*root.FulcioCertificateAuthority)
		if !ok {
			continue
		}
		if logKeyStatus(fca.ValidityPeriodStart, fca.ValidityPeriodEnd) == tuf.Expired {
			continue
		}
		if fca.Root != nil {
			roots.AddCert(fca.Root)
			rootsAdded++
		}
		for _, inter := range fca.Intermediates {
			if inter != nil {
				intermediates.AddCert(inter)
			}
		}
	}
	if rootsAdded == 0 {
		return nil, nil, fmt.Errorf("no Fulcio root certificates found in trusted root")
	}
	return roots, intermediates, nil
}

func publicKeyToPEM(pubKey interface{}) ([]byte, error) {
	derBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	}), nil
}

func logKeyStatus(start, end time.Time) tuf.StatusKind {
	now := time.Now()
	if !start.IsZero() && now.Before(start) {
		return tuf.Expired
	}
	if !end.IsZero() && now.After(end) {
		return tuf.Expired
	}
	return tuf.Active
}

// resolveTrustedMaterial returns the sigstore-go TrustedMaterial used to
// verify Sigstore bundles. When att.TrustedRoot carries an inline JSON value
// (typically resolved from a CEL expression reading a ConfigMap, see
// variables.CompiledAttestor), it is parsed and used directly instead of
// the trusted_root.json already fetched atomically by initTUFAndFetch. This
// allows verifying attestations/signatures from private Sigstore instances
// (e.g. GitHub's) that don't operate a TUF server, while still falling back
// to the TUF-derived trusted root for standard Sigstore verification.
func resolveTrustedMaterial(att *v1beta1.Cosign, defaultTrustedRoot *root.TrustedRoot) (root.TrustedMaterial, error) {
	if att.TrustedRoot != nil && att.TrustedRoot.Value != "" {
		if n := len(att.TrustedRoot.Value); n > maxTrustedRootJSONSize {
			return nil, fmt.Errorf("inline trustedRoot JSON is too large (%d bytes), maximum allowed is %d bytes", n, maxTrustedRootJSONSize)
		}
		tr, err := root.NewTrustedRootFromJSON([]byte(att.TrustedRoot.Value))
		if err != nil {
			return nil, fmt.Errorf("parsing inline trustedRoot JSON: %w", err)
		}
		return tr, nil
	}
	return defaultTrustedRoot, nil
}
