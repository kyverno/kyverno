package sigstoreguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// operatorRepository preserves the legacy operator configuration while moving
// its cache into sigstore-go's per-mirror layout. Policy repositories never call
// this helper. Imported metadata and targets are still verified by go-tuf/v2.
func operatorRepository(opts *tuf.Options, mirror string, explicitRoot bool, useCache bool) error {
	remote := tuf.DefaultMirror
	if !opts.DisableLocalCache {
		data, err := os.ReadFile(filepath.Join(opts.CachePath, "remote.json")) // #nosec G703 -- Fixed filename under the operator-controlled TUF cache; policy clients skip this helper.
		if err == nil {
			var cached struct {
				Mirror string `json:"mirror"`
			}
			if json.Unmarshal(data, &cached) == nil && cached.Mirror != "" {
				remote = cached.Mirror
			}
		}
	}
	// Only lazy default lookups reuse a previously initialized mirror. An
	// explicit startup configuration always takes precedence over remote.json.
	if mirror == "" && useCache {
		mirror = remote
	}
	if mirror == "" {
		mirror = tuf.DefaultMirror
	}
	opts.RepositoryBaseURL = operatorMirror(mirror)
	if opts.DisableLocalCache {
		return nil
	}

	directory := filepath.Join(opts.CachePath, tuf.URLToPath(opts.RepositoryBaseURL))
	rootPath := filepath.Join(directory, "root.json")
	if cachedRoot, err := os.ReadFile(rootPath); err == nil { // #nosec G703 -- Fixed root filename under the operator cache and filename-safe mirror directory; policy clients skip this helper.
		if !explicitRoot {
			opts.Root = cachedRoot
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading cached TUF root: %w", err)
	}
	// The old layout has only one repository. Never borrow its trust anchor
	// or metadata for a different mirror.
	if opts.RepositoryBaseURL != operatorMirror(remote) {
		return nil
	}
	metadata, err := legacyMetadata(filepath.Join(opts.CachePath, "tuf.db"))
	if err != nil {
		return err
	}
	if len(metadata["root.json"]) == 0 {
		return nil
	}
	if !explicitRoot {
		opts.Root = append([]byte(nil), metadata["root.json"]...)
	}
	return importLegacyCache(opts.CachePath, directory, metadata)
}

// The legacy client accepted a bare GCS bucket and mapped the public-good
// bucket aliases to the CDN. Keep those operator inputs working; policy inputs
// retain their strict HTTP(S) validation.
func operatorMirror(mirror string) string {
	switch mirror {
	case "sigstore-tuf-root", "https://sigstore-tuf-root.storage.googleapis.com", "https://storage.googleapis.com/sigstore-tuf-root":
		return tuf.DefaultMirror
	}
	if _, err := url.ParseRequestURI(mirror); err != nil {
		return "https://" + mirror + ".storage.googleapis.com"
	}
	return mirror
}

func legacyMetadata(path string) (map[string]json.RawMessage, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) { // #nosec G703 -- Caller supplies only the fixed tuf.db path under the operator-controlled cache.
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading legacy TUF cache: %w", err)
	}
	// Do not invoke the old store's writable open/recovery path: existing
	// installations may still use this database during a rolling upgrade.
	database, err := leveldb.OpenFile(path, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return nil, fmt.Errorf("opening legacy TUF cache: %w", err)
	}
	defer database.Close()
	metadata := make(map[string]json.RawMessage)
	iterator := database.NewIterator(nil, nil)
	defer iterator.Release()
	for iterator.Next() {
		metadata[string(iterator.Key())] = append(json.RawMessage(nil), iterator.Value()...)
	}
	if err := iterator.Error(); err != nil {
		return nil, fmt.Errorf("reading legacy TUF metadata: %w", err)
	}
	return metadata, nil
}

func importLegacyCache(cacheRoot, directory string, metadata map[string]json.RawMessage) error {
	if _, err := os.Stat(directory); err == nil { // #nosec G703 -- Destination is the filename-safe mirror directory under the operator-controlled TUF cache.
		// Never merge a legacy snapshot into a cache already owned by v2.
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.MkdirTemp(cacheRoot, ".tuf-migrate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for name, data := range metadata {
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		// go-tuf/v2 escapes role names in its local metadata filenames.
		name = url.PathEscape(strings.TrimSuffix(name, ".json")) + ".json"
		if err := os.WriteFile(filepath.Join(temporary, name), data, 0o600); err != nil { // #nosec G703 -- Metadata names are path-escaped into a private temporary directory in the operator cache.
			return err
		}
	}
	if err := copyLegacyTargets(filepath.Join(cacheRoot, "targets"), filepath.Join(temporary, "targets")); err != nil {
		return err
	}
	return os.Rename(temporary, directory) // #nosec G703 -- Both paths remain under the operator cache: a private temporary directory and the filename-safe mirror directory.
}

func copyLegacyTargets(source, destination string) error {
	if err := os.Mkdir(destination, 0o700); err != nil { // #nosec G703 -- Destination is the fixed targets directory inside the private operator-cache migration directory.
		return err
	}
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		data, err := root.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		// Both clients use url.PathEscape(targetName) for cached target files.
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), data, 0o600); err != nil { // #nosec G703 -- ReadDir supplies a single filename copied into the private operator-cache migration directory.
			return err
		}
	}
	return nil
}

// warmTargets retains Initialize's eager download behavior, including PEM-only
// repositories. Loading trusted_root.json alone would reject those mirrors.
func warmTargets(opts *tuf.Options) error {
	if !opts.DisableLocalCache {
		// The modern client has just authenticated any root rotations. Reuse
		// that accepted root when opening the updater for target enumeration.
		data, err := os.ReadFile(filepath.Join(opts.CachePath, tuf.URLToPath(opts.RepositoryBaseURL), "root.json"))
		if err != nil {
			return err
		}
		opts.Root = data
	}
	cached := *opts
	cached.ForceCache = true
	client, err := newUpdaterClientWithRootUpdate(&cached, false)
	if err != nil {
		return err
	}
	opts.Root = cached.Root
	for name := range client.GetTopLevelTargets() {
		if _, err := client.GetTarget(name); err != nil {
			return fmt.Errorf("warming TUF target %s: %w", name, err)
		}
	}
	return nil
}

func storeOperatorMirror(opts *tuf.Options) error {
	if opts.DisableLocalCache {
		return nil
	}
	data, err := json.Marshal(struct {
		Mirror string `json:"mirror"`
	}{Mirror: opts.RepositoryBaseURL})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(opts.CachePath, ".tuf-remote-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(opts.CachePath, "remote.json"))
}

// A refresh can authenticate a newer root. For cache-backed clients, advance the
// operation-local options so a subsequent PEM updater uses the same accepted root.
// Ordinary reads also retain it for offline default lookups, but startup must
// defer publishing until targets and mirror persistence have succeeded.
func retainRefreshedRoot(opts *tuf.Options, publishDefault bool) {
	if opts.DisableLocalCache {
		// Explicit policy clients always disable the local cache. This also
		// prevents their authenticated rotations from updating operator trust.
		return
	}
	data, err := os.ReadFile(filepath.Join(opts.CachePath, tuf.URLToPath(opts.RepositoryBaseURL), "root.json"))
	if err != nil {
		return
	}
	if publishDefault {
		// Compare the full configuration snapshot so a concurrent operator
		// change cannot be overwritten by a refresh of the previous default.
		mu.Lock()
		if defaultMirror == opts.RepositoryBaseURL && bytes.Equal(defaultRoot, opts.Root) {
			defaultRoot = append([]byte(nil), data...)
		}
		mu.Unlock()
	}
	opts.Root = data
}
