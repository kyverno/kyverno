// Package sigstoreguard loads Sigstore trust material through clients protected by
// Kyverno's egress policy. A process-wide lock serializes shared TUF cache access;
// policy-supplied mirrors use independent clients and cannot replace the
// controller's default trust configuration.
package sigstoreguard

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

var (
	mu            sync.Mutex
	cacheLock     = make(chan struct{}, 1)
	defaultMirror string
	defaultRoot   []byte
)

// SetDefaultRepository records the operator trust configuration for guarded
// verification without changing or refreshing the existing TUF singleton.
func SetDefaultRepository(mirror string, rootBytes []byte) {
	mu.Lock()
	defer mu.Unlock()
	defaultMirror = mirror
	defaultRoot = append([]byte(nil), rootBytes...)
}

// Initialize validates the controller's configured TUF repository before
// retaining it as the default. No global HTTP clients or transports are changed.
func Initialize(ctx context.Context, mirror string, rootBytes []byte) error {
	if err := lockSharedCache(ctx); err != nil {
		return err
	}
	defer unlockSharedCache()
	if _, err := newTUFClient(ctx, mirror, rootBytes, false); err != nil {
		return err
	}
	SetDefaultRepository(mirror, rootBytes)
	return nil
}

// TrustedRoot returns the default repository's trusted_root.json target.
func TrustedRoot(ctx context.Context) (*root.TrustedRoot, error) {
	return TrustedRootFor(ctx, "", nil)
}

// TrustedRootFor loads a policy's TUF repository without changing the default
// repository. The modern client supports delegated and succinct-role targets.
func TrustedRootFor(ctx context.Context, mirror string, rootBytes []byte) (*root.TrustedRoot, error) {
	isolate := mirror != "" || len(rootBytes) != 0
	if !isolate {
		mirror, rootBytes = defaultRepository()
		if err := lockSharedCache(ctx); err != nil {
			return nil, err
		}
		defer unlockSharedCache()
	}
	return trustedRoot(ctx, mirror, rootBytes, isolate)
}

func trustedRoot(ctx context.Context, mirror string, rootBytes []byte, isolate bool) (*root.TrustedRoot, error) {
	client, err := newTUFClientWithHTTP(ctx, mirror, rootBytes, true, registryclient.EgressHTTPClient(), isolate)
	if err != nil {
		return nil, fmt.Errorf("initializing TUF: %w", err)
	}
	target, err := client.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json from TUF: %w", err)
	}
	return root.NewTrustedRootFromJSON(target)
}

func newTUFClient(ctx context.Context, mirror string, rootBytes []byte, useCache bool) (*tuf.Client, error) {
	return newTUFClientWithHTTP(ctx, mirror, rootBytes, useCache, registryclient.EgressHTTPClient(), false)
}

func newTUFClientWithHTTP(ctx context.Context, mirror string, rootBytes []byte, useCache bool, httpClient *http.Client, isolate bool) (*tuf.Client, error) {
	opts, err := tufOptions(ctx, mirror, rootBytes, useCache, httpClient, isolate)
	if err != nil {
		return nil, err
	}
	return tuf.New(opts)
}

func tufOptions(ctx context.Context, mirror string, rootBytes []byte, useCache bool, httpClient *http.Client, isolate bool) (*tuf.Options, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f := fetcher.NewDefaultFetcher()
	f.SetHTTPClient(contextHTTPClient(func(req *http.Request) (*http.Response, error) {
		return httpClient.Do(req.WithContext(ctx)) //nolint:gosec // The injected transport validates and pins every network destination.
	}))
	opts := tuf.DefaultOptions().WithContext(ctx).WithFetcher(f)
	if mirror != "" {
		opts.RepositoryBaseURL = mirror
	}
	if len(rootBytes) != 0 {
		opts.Root = rootBytes
	}
	if cacheRoot := os.Getenv(legacytuf.TufRootEnv); cacheRoot != "" {
		opts.CachePath = cacheRoot
	}
	if useCache {
		opts.WithForceCache()
	}
	// A policy trust anchor must never read or overwrite another policy's
	// cache, including the operator's cache for the same mirror URL.
	disableCache, _ := strconv.ParseBool(os.Getenv(legacytuf.SigstoreNoCache))
	if isolate || disableCache {
		opts.WithDisableLocalCache()
	}
	if !isolate {
		mirrorURL, err := url.Parse(opts.RepositoryBaseURL)
		if err != nil {
			return nil, err
		}
		if mirrorURL.Scheme == "file" {
			opts.Fetcher = localFetcher(ctx, mirrorURL)
		}
	}
	return opts, nil
}

// legacyTUFClient exposes authenticated top-level target metadata for legacy Sigstore
// usage tags, while retaining go-tuf/v2 support for delegated targets.
type legacyTUFClient struct {
	*updater.Updater
}

func newUpdaterClient(opts *tuf.Options) (*legacyTUFClient, error) {
	cfg, err := config.New(opts.RepositoryBaseURL, opts.Root)
	if err != nil {
		return nil, err
	}
	cfg.LocalMetadataDir = filepath.Join(opts.CachePath, tuf.URLToPath(opts.RepositoryBaseURL))
	cfg.LocalTargetsDir = filepath.Join(cfg.LocalMetadataDir, "targets")
	cfg.DisableLocalCache = opts.DisableLocalCache
	cfg.Fetcher = opts.Fetcher
	cfg.PrefixTargetsWithHash = !opts.DisableConsistentSnapshot
	if opts.ForceCache && !opts.DisableLocalCache {
		// As in sigstore-go, reuse only complete, authenticated, unexpired
		// metadata. Missing or expired metadata requires a guarded refresh.
		local := *cfg
		local.UnsafeLocalMode = true
		cached, err := updater.New(&local)
		if err == nil && cached.Refresh() == nil {
			return &legacyTUFClient{cached}, nil
		}
	}
	client, err := updater.New(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Refresh(); err != nil {
		return nil, err
	}
	return &legacyTUFClient{client}, nil
}

func (c *legacyTUFClient) GetTarget(name string) ([]byte, error) {
	target, err := c.GetTargetInfo(name)
	if err != nil {
		return nil, err
	}
	path, data, err := c.FindCachedTarget(target, "")
	if err != nil {
		return nil, err
	}
	if path != "" {
		return data, nil
	}
	_, data, err = c.DownloadTarget(target, "", "")
	return data, err
}

// The TUF fetcher otherwise creates requests with context.Background(). Carry
// the verification deadline through every metadata and target download.
type contextHTTPClient func(*http.Request) (*http.Response, error)

func (c contextHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c(req)
}

// RekorPublicKeys returns keys from the default trust root, preserving the
// operator-configured local SIGSTORE_REKOR_PUBLIC_KEY override.
func RekorPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if file := os.Getenv("SIGSTORE_REKOR_PUBLIC_KEY"); file != "" {
		return publicKeysFromFile(file)
	}
	if keys, err := publicKeysFromTarget(ctx, legacytuf.Rekor, "rekor.pub"); err == nil {
		return keys, nil
	}
	tr, err := TrustedRoot(ctx)
	if err != nil {
		return nil, err
	}
	return RekorKeysFromTrustedRoot(tr)
}

// CTLogPublicKeys returns keys from the default trust root, preserving the
// operator-configured local SIGSTORE_CT_LOG_PUBLIC_KEY_FILE override.
func CTLogPublicKeys(ctx context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
	if file := os.Getenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE"); file != "" {
		return publicKeysFromFile(file)
	}
	if keys, err := publicKeysFromTarget(ctx, legacytuf.CTFE, "ctfe.pub"); err == nil {
		return keys, nil
	}
	tr, err := TrustedRoot(ctx)
	if err != nil {
		return nil, err
	}
	return CTLogKeysFromTrustedRoot(tr)
}

func publicKeysFromFile(file string) (*cosign.TrustedTransparencyLogPubKeys, error) {
	data, err := os.ReadFile(file) //nolint:gosec // The path is an operator-configured local key override, never policy input.
	if err != nil {
		return nil, err
	}
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	if err := keys.AddTransparencyLogPubKey(data, legacytuf.Active); err != nil {
		return nil, err
	}
	return &keys, nil
}

// FulcioRoots returns root and intermediate certificates from the default trust
// root. It does not use Sigstore's unguarded global TUF singleton.
func FulcioRoots() (*x509.CertPool, *x509.CertPool, error) {
	return FulcioRootsWithContext(context.Background())
}

// FulcioRootsWithContext preserves the caller's verification deadline while
// retaining historical CAs needed to validate certificates at signing time.
func FulcioRootsWithContext(ctx context.Context) (*x509.CertPool, *x509.CertPool, error) {
	if roots, intermediates, err := fulcioRootsFromTargets(ctx); err == nil {
		return roots, intermediates, nil
	}
	tr, err := TrustedRoot(ctx)
	if err != nil {
		return nil, nil, err
	}
	return fulcioRootsFromTrustedRoot(tr, false)
}

// WithLock serializes access to the shared TUF cache. The callback must not call
// another function in this package that acquires the same lock.
func WithLock(fn func() error) error {
	if err := lockSharedCache(context.Background()); err != nil {
		return err
	}
	defer unlockSharedCache()
	return fn()
}

// Operator defaults are copied under a short lock so isolated downloads never
// hold up readers or configuration changes. Cache access has its own lock.
func defaultRepository() (string, []byte) {
	mu.Lock()
	defer mu.Unlock()
	return defaultMirror, append([]byte(nil), defaultRoot...)
}

func lockSharedCache(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case cacheLock <- struct{}{}:
		if err := ctx.Err(); err != nil {
			unlockSharedCache()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func unlockSharedCache() {
	<-cacheLock
}

// defaultTargets preserves legacy repositories that publish individual PEM
// targets instead of trusted_root.json, using the same guarded TUF client.
func defaultTargets(ctx context.Context, usage legacytuf.UsageKind, names ...string) ([]legacytuf.TargetFile, error) {
	mirror, rootBytes := defaultRepository()
	if err := lockSharedCache(ctx); err != nil {
		return nil, err
	}
	defer unlockSharedCache()
	opts, err := tufOptions(ctx, mirror, rootBytes, true, registryclient.EgressHTTPClient(), false)
	if err != nil {
		return nil, err
	}
	client, err := newUpdaterClient(opts)
	if err != nil {
		return nil, err
	}
	return targetsFromClient(client, usage, names...)
}

func targetsFromClient(client *legacyTUFClient, usage legacytuf.UsageKind, names ...string) ([]legacytuf.TargetFile, error) {
	var targets []legacytuf.TargetFile
	for name, info := range client.GetTopLevelTargets() {
		if info.Custom == nil {
			continue
		}
		var custom struct {
			Sigstore struct {
				Usage  legacytuf.UsageKind  `json:"usage"`
				Status legacytuf.StatusKind `json:"status"`
			} `json:"sigstore"`
		}
		if err := json.Unmarshal(*info.Custom, &custom); err != nil || custom.Sigstore.Usage != usage {
			continue
		}
		target, err := client.GetTarget(name)
		if err != nil {
			return nil, err
		}
		targets = append(targets, legacytuf.TargetFile{Name: name, Target: target, Status: custom.Sigstore.Status})
	}
	if len(targets) != 0 {
		return targets, nil
	}
	var lastErr error
	for _, name := range names {
		target, err := client.GetTarget(name)
		if err != nil {
			lastErr = err
			continue
		}
		targets = append(targets, legacytuf.TargetFile{Name: name, Target: target, Status: legacytuf.Active})
	}
	if len(targets) == 0 {
		return nil, lastErr
	}
	return targets, nil
}

func publicKeysFromTarget(ctx context.Context, usage legacytuf.UsageKind, name string) (*cosign.TrustedTransparencyLogPubKeys, error) {
	targets, err := defaultTargets(ctx, usage, name)
	if err != nil {
		return nil, err
	}
	return publicKeysFromTargets(targets)
}

func publicKeysFromTargets(targets []legacytuf.TargetFile) (*cosign.TrustedTransparencyLogPubKeys, error) {
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	for _, target := range targets {
		if err := keys.AddTransparencyLogPubKey(target.Target, target.Status); err != nil {
			return nil, err
		}
	}
	return &keys, nil
}

func fulcioRootsFromTargets(ctx context.Context) (*x509.CertPool, *x509.CertPool, error) {
	targets, err := defaultTargets(ctx, legacytuf.Fulcio, "fulcio.crt.pem", "fulcio_v1.crt.pem", "fulcio_intermediate_v1.crt.pem")
	if err != nil {
		return nil, nil, err
	}
	return fulcioRootsFromTargetFiles(targets)
}

func fulcioRootsFromTargetFiles(targets []legacytuf.TargetFile) (*x509.CertPool, *x509.CertPool, error) {
	roots, intermediates := x509.NewCertPool(), x509.NewCertPool()
	rootsAdded := 0
	for _, target := range targets {
		certs, err := cryptoutils.UnmarshalCertificatesFromPEM(target.Target)
		if err != nil {
			return nil, nil, err
		}
		for _, cert := range certs {
			if bytes.Equal(cert.RawSubject, cert.RawIssuer) {
				roots.AddCert(cert)
				rootsAdded++
			} else {
				intermediates.AddCert(cert)
			}
		}
	}
	if rootsAdded == 0 {
		return nil, nil, fmt.Errorf("no Fulcio root certificates found in legacy targets")
	}
	return roots, intermediates, nil
}
