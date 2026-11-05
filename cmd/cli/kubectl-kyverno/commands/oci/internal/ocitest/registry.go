// Package ocitest holds registry test support shared across the oci subcommand packages'
// own test files. It is test support only; nothing here is imported by non-test code.
package ocitest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// NewRegistry starts an in-process, anonymous-access OCI registry and returns its host:port,
// closing the server when t's test ends. It replaces the inline
// httptest.NewServer(registry.New()) copy duplicated across pull, validate, inspect, and resolve's
// own test files.
func NewRegistry(t *testing.T) (host string) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host
}

// NewRecordingRegistry works like NewRegistry, but also returns a function that lists every
// request path the registry has received since the last reset, in order, and a function to reset
// that log. It lets a test push a fixture (whose own blob-existence checks and uploads would
// otherwise pollute the log), reset, and then assert that a specific read only touched the blobs
// it should have.
func NewRecordingRegistry(t *testing.T) (host string, requests func() []string, reset func()) {
	t.Helper()
	var mu sync.Mutex
	var paths []string

	handler := registry.New()
	recorder := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		handler.ServeHTTP(w, r)
	})

	srv := httptest.NewServer(recorder)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host,
		func() []string {
			mu.Lock()
			defer mu.Unlock()
			out := make([]string, len(paths))
			copy(out, paths)
			return out
		},
		func() {
			mu.Lock()
			defer mu.Unlock()
			paths = nil
		}
}

// NewAuthRegistry starts an in-process registry that requires HTTP Basic auth matching user/pass
// on every request, including the /v2/ ping: go-containerregistry's remote client only attaches
// credentials once it has seen a 401 on that first request, so a wrapper that only guarded
// deeper paths would never be exercised. A request with no Authorization header, or one that
// decodes to the wrong username or password, gets a 401 with a WWW-Authenticate challenge; this
// checks the decoded credentials, not merely that the header is present, so a wrapper that
// forgot to compare them would fail TestAuthRegistryRejectsWrongPassword elsewhere. Only Basic is
// emulated: Bearer (used by GHCR, ECR, Docker Hub, and ACR) is not something registry.New can
// stand in for, and is untested by this helper.
func NewAuthRegistry(t *testing.T, user, pass string) (host string) {
	t.Helper()
	handler := registry.New()
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, ok := r.BasicAuth()
		if !ok || gotUser != user || gotPass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="kyverno-test"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host
}

// StaticKeychain returns an authn.Keychain that resolves exactly host to a fixed Basic
// credential and everything else to anonymous. It matches on the resource's registry host
// (RegistryStr, the "127.0.0.1:port" form an in-process registry presents), not its full name,
// so the credential applies to every repository on that host.
func StaticKeychain(host, user, pass string) authn.Keychain {
	return &staticKeychain{host: host, user: user, pass: pass}
}

type staticKeychain struct {
	host, user, pass string
}

func (k *staticKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if target.RegistryStr() != k.host {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{Username: k.user, Password: k.pass}), nil
}

// epoch is the content layer's fixed tar/gzip mtime, matching internal/bundle's own
// deterministic-tar convention (bundle-spec.md section 5), so content PushTree pushes is
// byte-identical to what the real writer would produce for the same files.
var epoch = time.Unix(0, 0).UTC()

// bundleConfig, configSet, and resourceEntry mirror the JSON shape of internal/bundle.Config,
// ConfigSet, and ResourceEntry closely enough for Read's crossCheckIndex (every archived
// document has a matching config index entry with the same digest, apiVersion, kind, namespace,
// and name, and vice versa) to pass over PushTree's own content layer. PushTree is test support
// for pushing a structurally valid bundle quickly; a test that needs push's own strict validation
// (CEL compilation, kind/version checks, exception resolution) builds its image through the
// internal/bundle package directly instead, the way resolve and validate's own test files
// already do.
type bundleConfig struct {
	FormatVersion string          `json:"formatVersion"`
	Sets          []configSet     `json:"sets"`
	Resources     []resourceEntry `json:"resources"`
}

type configSet struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type resourceEntry struct {
	APIVersion    string `json:"apiVersion"`
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DocumentIndex int    `json:"documentIndex"`
	Digest        string `json:"digest"`
	Set           string `json:"set"`
}

// typeMeta is the subset of a document's fields buildResourceIndex reads, mirroring
// internal/bundle.Assemble's own typeMeta.
type typeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// buildResourceIndex scans every file's YAML documents and builds the resources/sets pair a
// bundle config needs for crossCheckIndex, assigning each document to the "exceptions" or
// "policies" set the same way internal/bundle's writer does (PolicyException goes to
// exceptions, everything else to policies).
func buildResourceIndex(t *testing.T, files map[string]string) ([]resourceEntry, []configSet) {
	t.Helper()
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var resources []resourceEntry
	setTypes := map[string]string{}
	for _, p := range paths {
		docs, err := extyaml.SplitDocuments([]byte(files[p]))
		require.NoError(t, err)
		for i, doc := range docs {
			if extyaml.IsEmptyDocument(doc) {
				continue
			}
			var tm typeMeta
			require.NoError(t, yaml.Unmarshal(doc, &tm))
			set := "policies"
			if tm.Kind == "PolicyException" {
				set = "exceptions"
			}
			setTypes[set] = set
			sum := sha256.Sum256(doc)
			resources = append(resources, resourceEntry{
				APIVersion:    tm.APIVersion,
				Kind:          tm.Kind,
				Namespace:     tm.Metadata.Namespace,
				Name:          tm.Metadata.Name,
				Path:          p,
				DocumentIndex: i,
				Digest:        "sha256:" + hex.EncodeToString(sum[:]),
				Set:           set,
			})
		}
	}
	sets := make([]configSet, 0, len(setTypes))
	for name, typ := range setTypes {
		sets = append(sets, configSet{Name: name, Type: typ})
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Name < sets[j].Name })
	return resources, sets
}

// manifestTaggable is the minimal remote.Taggable so a hand-built manifest can be pushed with
// remote.Put without constructing a full v1.Image.
type manifestTaggable struct {
	raw []byte
	mt  types.MediaType
}

func (m manifestTaggable) RawManifest() ([]byte, error)        { return m.raw, nil }
func (m manifestTaggable) MediaType() (types.MediaType, error) { return m.mt, nil }

// PushTree pushes files as a structurally valid format-1.0 bundle (bundle-spec.md sections 4-6:
// one deterministic tar+gzip content layer, a JSON config with formatVersion "1.0.0", an OCI 1.0
// image manifest) to ref using kc, returning the pushed manifest's digest. It builds the image
// directly from go-containerregistry primitives rather than importing internal/bundle: ocitest
// sits under internal/, and internal/bundle's own test files already import ocitest (for
// NewRecordingRegistry), so the reverse import would be a cycle. A test that needs push's
// validation rules exercised builds its image through internal/bundle directly instead.
func PushTree(t *testing.T, ref string, files map[string]string, kc authn.Keychain) v1.Hash {
	t.Helper()

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for _, p := range paths {
		content := files[p]
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     p,
			Size:     int64(len(content)),
			Mode:     0o644,
			ModTime:  epoch,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, err := gw.Write(tarBuf.Bytes())
	require.NoError(t, err)
	require.NoError(t, gw.Close())

	contentLayer := static.NewLayer(gzBuf.Bytes(), types.MediaType(internal.ContentMediaType))
	resources, sets := buildResourceIndex(t, files)
	configBytes, err := json.Marshal(bundleConfig{
		FormatVersion: "1.0.0",
		Sets:          sets,
		Resources:     resources,
	})
	require.NoError(t, err)
	configLayer := static.NewLayer(configBytes, types.MediaType(internal.ConfigMediaType))

	layerDigest, err := contentLayer.Digest()
	require.NoError(t, err)
	layerSize, err := contentLayer.Size()
	require.NoError(t, err)
	configDigest, err := configLayer.Digest()
	require.NoError(t, err)
	configSize, err := configLayer.Size()
	require.NoError(t, err)

	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config: v1.Descriptor{
			MediaType: types.MediaType(internal.ConfigMediaType),
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: []v1.Descriptor{{
			MediaType: types.MediaType(internal.ContentMediaType),
			Digest:    layerDigest,
			Size:      layerSize,
		}},
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)

	name, err := gcrname.ParseReference(ref)
	require.NoError(t, err)
	repo := name.Context()

	// A nil kc means anonymous: remote.WithAuthFromKeychain(nil) would panic on the first
	// Resolve call, so the option is only added when a keychain was actually supplied.
	var opts []remote.Option
	if kc != nil {
		opts = append(opts, remote.WithAuthFromKeychain(kc))
	}
	require.NoError(t, remote.WriteLayer(repo, contentLayer, opts...))
	require.NoError(t, remote.WriteLayer(repo, configLayer, opts...))
	require.NoError(t, remote.Put(name, manifestTaggable{raw: manifestBytes, mt: manifest.MediaType}, opts...))

	digest, _, err := v1.SHA256(bytes.NewReader(manifestBytes))
	require.NoError(t, err)
	return digest
}
