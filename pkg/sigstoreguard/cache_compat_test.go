package sigstoreguard

import (
	"context"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestLegacyOperatorCacheMigration(t *testing.T) {
	cacheRoot, mirror, rootBytes, target := legacyCacheFixture(t)
	before, err := legacyMetadata(filepath.Join(cacheRoot, "tuf.db"))
	require.NoError(t, err)
	requests := 0
	client, err := newTUFClientWithHTTP(context.Background(), "", nil, true, missingTUFClient(&requests), false)
	require.NoError(t, err)
	actual, err := client.GetTarget("trusted_root.json")
	require.NoError(t, err)
	require.Equal(t, target, actual)
	require.Zero(t, requests, "a complete, unexpired legacy cache must remain usable offline")
	directory := filepath.Join(cacheRoot, tuf.URLToPath(mirror))
	root, err := os.ReadFile(filepath.Join(directory, "root.json"))
	require.NoError(t, err)
	require.Equal(t, rootBytes, root)
	after, err := legacyMetadata(filepath.Join(cacheRoot, "tuf.db"))
	require.NoError(t, err)
	require.Equal(t, before, after, "migration must not rewrite the legacy database")
	legacyTarget, err := os.ReadFile(filepath.Join(cacheRoot, "targets", "trusted_root.json"))
	require.NoError(t, err)
	require.Equal(t, target, legacyTarget)
}

func TestLegacyCacheCannotOverrideExplicitTrust(t *testing.T) {
	for _, kind := range []string{"different_mirror", "explicit_root", "isolated_policy", "no_cache"} {
		t.Run(kind, func(t *testing.T) {
			cacheRoot, mirror, _, _ := legacyCacheFixture(t)
			selectedMirror := ""
			var explicitRoot []byte
			isolate := false
			switch kind {
			case "different_mirror":
				selectedMirror = "https://other.example"
			case "explicit_root":
				explicitRoot = tuf.DefaultRoot()
			case "isolated_policy":
				selectedMirror, isolate = mirror, true
			case "no_cache":
				t.Setenv("SIGSTORE_NO_CACHE", "true")
			}
			opts, err := tufOptions(context.Background(), selectedMirror, explicitRoot, true, http.DefaultClient, isolate)
			require.NoError(t, err)
			require.Equal(t, tuf.DefaultRoot(), opts.Root, "operator cache must not replace the selected root")
			if kind != "explicit_root" {
				require.NoDirExists(t, filepath.Join(cacheRoot, tuf.URLToPath(mirror)))
			}
			if kind == "different_mirror" {
				require.Equal(t, selectedMirror, opts.RepositoryBaseURL)
			}
			if kind == "no_cache" {
				require.Equal(t, tuf.DefaultMirror, opts.RepositoryBaseURL)
				require.True(t, opts.DisableLocalCache)
			}
		})
	}
}

func TestLegacyCacheImportedTargetsAreAuthenticated(t *testing.T) {
	cacheRoot, _, _, _ := legacyCacheFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "targets", "trusted_root.json"), []byte("untrusted target"), 0o600))
	requests := 0
	client, err := newTUFClientWithHTTP(context.Background(), "", nil, true, missingTUFClient(&requests), false)
	require.NoError(t, err)
	_, err = client.GetTarget("trusted_root.json")
	require.Error(t, err, "a target with a mismatched signed hash must not be returned")
	require.Positive(t, requests, "the client must try a guarded download after rejecting the bad target")
}

func TestLegacyCacheImportedMetadataIsAuthenticated(t *testing.T) {
	cacheRoot, _, _, _ := legacyCacheFixture(t)
	database, err := leveldb.OpenFile(filepath.Join(cacheRoot, "tuf.db"), nil)
	require.NoError(t, err)
	require.NoError(t, database.Put([]byte("targets.json"), []byte(`{"signed":{"_type":"targets"},"signatures":[]}`), nil))
	require.NoError(t, database.Close())
	requests := 0
	_, err = newTUFClientWithHTTP(context.Background(), "", nil, true, missingTUFClient(&requests), false)
	require.Error(t, err, "unsigned metadata must not become trusted during migration")
	require.Positive(t, requests, "invalid cached metadata must require a guarded refresh")
}

func TestOperatorMirrorCompatibility(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"sigstore-tuf-root", tuf.DefaultMirror},
		{"https://sigstore-tuf-root.storage.googleapis.com", tuf.DefaultMirror},
		{"https://storage.googleapis.com/sigstore-tuf-root", tuf.DefaultMirror},
		{"private-tuf-bucket", "https://private-tuf-bucket.storage.googleapis.com"},
		{"https://mirror.example/tuf", "https://mirror.example/tuf"},
		{"file:///tmp/tuf", "file:///tmp/tuf"},
	} {
		t.Run(test.input, func(t *testing.T) {
			require.Equal(t, test.want, operatorMirror(test.input))
		})
	}
	t.Setenv("TUF_ROOT", t.TempDir())
	opts, err := tufOptions(context.Background(), "private-tuf-bucket", nil, true, http.DefaultClient, true)
	require.NoError(t, err)
	require.Equal(t, "private-tuf-bucket", opts.RepositoryBaseURL, "policy mirror validation must not be relaxed")
}

func TestOperatorRepositoryPersistsSuccessfulConfiguration(t *testing.T) {
	cacheRoot := t.TempDir()
	opts := tuf.DefaultOptions().WithCachePath(cacheRoot).WithRepositoryBaseURL("https://private.example/tuf")
	require.NoError(t, storeOperatorMirror(opts))
	next := tuf.DefaultOptions().WithCachePath(cacheRoot)
	require.NoError(t, operatorRepository(next, "", false, true))
	require.Equal(t, opts.RepositoryBaseURL, next.RepositoryBaseURL)
	// Startup's explicit public default takes precedence over the old mirror.
	next = tuf.DefaultOptions().WithCachePath(cacheRoot)
	require.NoError(t, operatorRepository(next, "", false, false))
	require.Equal(t, tuf.DefaultMirror, next.RepositoryBaseURL)
	// Disabling cache must neither consume nor change that saved setting.
	opts.WithDisableLocalCache().WithRepositoryBaseURL("https://uncached.example")
	require.NoError(t, storeOperatorMirror(opts))
	next = tuf.DefaultOptions().WithCachePath(cacheRoot)
	require.NoError(t, operatorRepository(next, "", false, true))
	require.Equal(t, "https://private.example/tuf", next.RepositoryBaseURL)
}

func TestOperatorInitializationWarmsTargets(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	t.Setenv("TUF_ROOT", t.TempDir())
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	fixture := newVerificationFixture(t)
	rootBytes, mirror, httpClient := fixture.repository(t, true)
	require.NoError(t, initializeWithHTTP(context.Background(), mirror, rootBytes, httpClient))
	configuredMirror, configuredRoot := defaultRepository()
	require.Equal(t, mirror, configuredMirror)
	require.Equal(t, rootBytes, configuredRoot)
	requests := 0
	client, err := newTUFClientWithHTTP(context.Background(), mirror, rootBytes, true, missingTUFClient(&requests), false)
	require.NoError(t, err)
	for _, name := range []string{"trusted_root.json", "rekor-previous.pub", "ctfe-previous.pub", "fulcio-previous.pem"} {
		_, err := client.GetTarget(name)
		require.NoError(t, err, "startup must cache published target %s", name)
	}
	require.Zero(t, requests)
	// A failed initialization must preserve both the active configuration and
	// the persisted mirror, even if the attempted repository has a valid root.
	require.Error(t, initializeWithHTTP(context.Background(), "https://unavailable.example", rootBytes, missingTUFClient(&requests)))
	configuredMirror, configuredRoot = defaultRepository()
	require.Equal(t, mirror, configuredMirror)
	require.Equal(t, rootBytes, configuredRoot)
	next := tuf.DefaultOptions().WithCachePath(os.Getenv("TUF_ROOT"))
	require.NoError(t, operatorRepository(next, "", false, true))
	require.Equal(t, mirror, next.RepositoryBaseURL)
}

func TestOperatorInitializationWithoutCache(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	cacheRoot := t.TempDir()
	t.Setenv("TUF_ROOT", cacheRoot)
	t.Setenv("SIGSTORE_NO_CACHE", "true")
	fixture := newVerificationFixture(t)
	rootBytes, mirror, httpClient := fixture.repository(t, true)
	require.NoError(t, initializeWithHTTP(context.Background(), mirror, rootBytes, httpClient))
	entries, err := os.ReadDir(cacheRoot)
	require.NoError(t, err)
	require.Empty(t, entries, "SIGSTORE_NO_CACHE must prevent metadata, target, and mirror cache writes")
}

func TestOperatorInitializationPublishesRotatedRootOnlyAfterSuccess(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	for _, failure := range []string{"target_download", "mirror_persistence", "none"} {
		t.Run(failure, func(t *testing.T) {
			cacheRoot := t.TempDir()
			t.Setenv("TUF_ROOT", cacheRoot)
			t.Setenv("SIGSTORE_NO_CACHE", "false")
			initial, rotated, mirror, _, httpClient := rotatingRepository(t)
			SetDefaultRepository(mirror, initial)
			remotePath := filepath.Join(cacheRoot, "remote.json")
			if failure == "mirror_persistence" {
				// An existing directory prevents the atomic mirror-file rename,
				// after metadata refresh and all target downloads have succeeded.
				require.NoError(t, os.Mkdir(remotePath, 0o700))
			}
			transport := httpClient.Transport
			targetRequests := 0
			httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if strings.HasPrefix(req.URL.Path, "/targets/") {
					targetRequests++
					configuredMirror, configuredRoot := defaultRepository()
					require.Equal(t, mirror, configuredMirror)
					require.JSONEq(t, string(initial), string(configuredRoot), "warming must not publish the pending root")
					if failure == "target_download" {
						return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("unavailable target")), Request: req}, nil
					}
				}
				return transport.RoundTrip(req)
			})
			err := initializeWithHTTP(context.Background(), mirror, initial, httpClient)
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Positive(t, targetRequests, "the failure must occur after authenticated root rotation")
			cachedRoot, err := os.ReadFile(filepath.Join(cacheRoot, tuf.URLToPath(mirror), "root.json"))
			require.NoError(t, err)
			require.JSONEq(t, string(rotated), string(cachedRoot))
			configuredMirror, configuredRoot := defaultRepository()
			require.Equal(t, mirror, configuredMirror)
			if failure == "none" {
				require.JSONEq(t, string(rotated), string(configuredRoot))
			} else {
				require.JSONEq(t, string(initial), string(configuredRoot), "a failed initialization must retain the active root")
			}
		})
	}
}

func TestDefaultRootRotationRetainsOfflineCache(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	for _, kind := range []string{"trusted_root", "legacy_targets"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("TUF_ROOT", t.TempDir())
			t.Setenv("SIGSTORE_NO_CACHE", "false")
			initial, rotated, mirror, target, httpClient := rotatingRepository(t)
			SetDefaultRepository(mirror, initial)
			opts, err := tufOptions(context.Background(), mirror, initial, false, httpClient, false)
			require.NoError(t, err)
			var client interface{ GetTarget(string) ([]byte, error) }
			if kind == "legacy_targets" {
				client, err = newUpdaterClient(opts)
			} else {
				client, err = newTUFClientWithOptions(opts)
			}
			require.NoError(t, err)
			actual, err := client.GetTarget("trusted_root.json")
			require.NoError(t, err)
			require.Equal(t, target, actual)
			retainedMirror, retainedRoot := defaultRepository()
			require.Equal(t, mirror, retainedMirror)
			require.JSONEq(t, string(rotated), string(retainedRoot))
			requests := 0
			// Modern and PEM clients may reuse the same options. The second
			// client must retain the first client's accepted rotation, even after
			// the mirror goes offline.
			opts.ForceCache = true
			opts.Fetcher = localFetchFunc(func(rawURL string, _ int64) ([]byte, error) {
				requests++
				return nil, &metadata.ErrDownloadHTTP{StatusCode: http.StatusNotFound, URL: rawURL}
			})
			pemClient, err := newUpdaterClient(opts)
			require.NoError(t, err)
			actual, err = pemClient.GetTarget("trusted_root.json")
			require.NoError(t, err)
			require.Equal(t, target, actual)
			require.Zero(t, requests, "both clients in one verification must use the accepted root")
			next, err := newTUFClientWithHTTP(context.Background(), retainedMirror, retainedRoot, true, missingTUFClient(&requests), false)
			require.NoError(t, err)
			actual, err = next.GetTarget("trusted_root.json")
			require.NoError(t, err)
			require.Equal(t, target, actual)
			require.Zero(t, requests, "authenticated root rotation must not force every subsequent lookup online")
		})
	}
}

func rotatingRepository(t *testing.T) ([]byte, []byte, string, []byte, *http.Client) {
	t.Helper()
	first, second := newVerificationFixture(t), newVerificationFixture(t)
	oldSigner, err := signature.LoadSignerVerifier(first.key, crypto.SHA256)
	require.NoError(t, err)
	newSigner, err := signature.LoadSignerVerifier(second.key, crypto.SHA256)
	require.NoError(t, err)
	expires := time.Now().Add(time.Hour)
	makeRoot := func(fixture *verificationFixture, version int64) *metadata.Metadata[metadata.RootType] {
		key, err := metadata.KeyFromPublicKey(&fixture.key.PublicKey)
		require.NoError(t, err)
		root := metadata.Root(expires)
		root.Signed.Version = version
		for _, role := range []string{metadata.ROOT, metadata.TIMESTAMP, metadata.SNAPSHOT, metadata.TARGETS} {
			require.NoError(t, root.Signed.AddKey(key, role))
		}
		return root
	}
	initial, rotated := makeRoot(first, 1), makeRoot(second, 2)
	_, err = initial.Sign(oldSigner)
	require.NoError(t, err)
	_, err = rotated.Sign(oldSigner)
	require.NoError(t, err)
	_, err = rotated.Sign(newSigner)
	require.NoError(t, err)
	initialBytes, err := initial.ToBytes(false)
	require.NoError(t, err)
	rotatedBytes, err := rotated.ToBytes(false)
	require.NoError(t, err)
	target, err := second.trustedRoot.MarshalJSON()
	require.NoError(t, err)
	info, err := metadata.TargetFile().FromBytes("trusted_root.json", target, "sha256")
	require.NoError(t, err)
	targets := metadata.Targets(expires)
	targets.Signed.Targets["trusted_root.json"] = info
	files := map[string][]byte{
		"/2.root.json": rotatedBytes,
		"/targets/" + hex.EncodeToString(info.Hashes["sha256"]) + ".trusted_root.json": target,
	}
	type signedMetadata interface {
		Sign(signature.Signer) (*metadata.Signature, error)
		ToBytes(bool) ([]byte, error)
	}
	for path, document := range map[string]signedMetadata{
		"/timestamp.json":  metadata.Timestamp(expires),
		"/1.snapshot.json": metadata.Snapshot(expires),
		"/1.targets.json":  targets,
	} {
		_, err := document.Sign(newSigner)
		require.NoError(t, err)
		files[path], err = document.ToBytes(false)
		require.NoError(t, err)
	}
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		data, ok := files[req.URL.Path]
		status := http.StatusOK
		if !ok {
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(data))), Request: req}, nil
	})}
	return initialBytes, rotatedBytes, "https://rotate.example", target, httpClient
}

func legacyCacheFixture(t *testing.T) (string, string, []byte, []byte) {
	t.Helper()
	cacheRoot := t.TempDir()
	t.Setenv("TUF_ROOT", cacheRoot)
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	fixture := newVerificationFixture(t)
	rootBytes, mirror, httpClient := fixture.repository(t, true)
	client, err := newTUFClientWithHTTP(context.Background(), mirror, rootBytes, false, httpClient, false)
	require.NoError(t, err)
	target, err := client.GetTarget("trusted_root.json")
	require.NoError(t, err)
	directory := filepath.Join(cacheRoot, tuf.URLToPath(mirror))
	database, err := leveldb.OpenFile(filepath.Join(cacheRoot, "tuf.db"), nil)
	require.NoError(t, err)
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		require.NoError(t, err)
		require.NoError(t, database.Put([]byte(name), data, nil))
	}
	require.NoError(t, database.Close())
	require.NoError(t, os.Mkdir(filepath.Join(cacheRoot, "targets"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "targets", "trusted_root.json"), target, 0o600))
	remote, err := json.Marshal(map[string]string{"mirror": mirror})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "remote.json"), remote, 0o600))
	require.NoError(t, os.RemoveAll(directory))
	return cacheRoot, mirror, rootBytes, target
}

func missingTUFClient(requests *int) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*requests++
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(fmt.Sprintf("missing %s", req.URL.Path))), Request: req}, nil
	})}
}
