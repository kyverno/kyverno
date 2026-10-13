package sigstoreguard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/cosign/v3/pkg/cosign"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestLogKeyRepositoriesRejectFailedDeclaredTargets(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	t.Setenv("SIGSTORE_NO_CACHE", "true")
	for _, log := range []struct {
		name        string
		tagged      string
		environment string
		read        func(context.Context) (*cosign.TrustedTransparencyLogPubKeys, error)
	}{
		{"rekor.pub", "rekor-previous.pub", "SIGSTORE_REKOR_PUBLIC_KEY", RekorPublicKeys},
		{"ctfe.pub", "ctfe-previous.pub", "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", CTLogPublicKeys},
	} {
		for _, test := range []struct {
			name       string
			failure    string
			untagged   bool
			modernOnly bool
			override   bool
		}{
			{name: "mixed"},
			{name: "modern_only", modernOnly: true},
			{name: "legacy_only", failure: "modern_absent"},
			{name: "preferred_legacy_with_invalid_modern", failure: "modern_invalid"},
			{name: "tagged_missing", failure: "download"},
			{name: "tagged_hash", failure: "hash"},
			{name: "tagged_pem", failure: "malformed"},
			{name: "tagged_empty", failure: "empty"},
			{name: "untagged_missing", failure: "download", untagged: true},
			{name: "untagged_hash", failure: "hash", untagged: true},
			{name: "untagged_pem", failure: "malformed", untagged: true},
			{name: "operator_override", failure: "download", override: true},
		} {
			t.Run(log.name+"/"+test.name, func(t *testing.T) {
				t.Setenv("TUF_ROOT", t.TempDir())
				t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
				t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
				fixture := newVerificationFixture(t)
				var files map[string][]byte
				rootBytes, _, _ := fixture.repositoryWithChanges(t, !test.modernOnly, func(targets *metadata.Metadata[metadata.TargetsType], repository map[string][]byte) {
					files = repository
					name := log.tagged
					if test.untagged {
						info := targets.Signed.Targets[name]
						data := files[fixtureTargetPath(name, info)]
						delete(targets.Signed.Targets, name)
						delete(files, fixtureTargetPath(name, info))
						name = log.name
						replacement, err := metadata.TargetFile().FromBytes(name, data, "sha256")
						require.NoError(t, err)
						targets.Signed.Targets[name] = replacement
						files[fixtureTargetPath(name, replacement)] = data
					}
					if test.failure == "" {
						return
					}
					if test.failure == "modern_absent" || test.failure == "modern_invalid" {
						name = "trusted_root.json"
					}
					info := targets.Signed.Targets[name]
					path := fixtureTargetPath(name, info)
					switch test.failure {
					case "modern_absent":
						delete(targets.Signed.Targets, name)
						delete(files, path)
					case "download":
						delete(files, path)
					case "hash":
						files[path] = bytes.Repeat([]byte("x"), len(files[path]))
					case "malformed", "empty", "modern_invalid":
						data := []byte("invalid trust material")
						if test.failure == "empty" {
							data = nil
						}
						replacement, err := metadata.TargetFile().FromBytes(name, data, "sha256")
						require.NoError(t, err)
						replacement.Custom = info.Custom
						targets.Signed.Targets[name] = replacement
						delete(files, path)
						files[fixtureTargetPath(name, replacement)] = data
					}
				})
				directory := t.TempDir()
				for name, data := range files {
					path := filepath.Join(directory, name)
					require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
					require.NoError(t, os.WriteFile(path, data, 0o600))
				}
				SetDefaultRepository("file://"+directory, rootBytes)
				if test.override {
					file := filepath.Join(t.TempDir(), "override.pub")
					require.NoError(t, os.WriteFile(file, fixture.legacyPEM, 0o600))
					t.Setenv(log.environment, file)
				}
				keys, err := log.read(context.Background())
				wantError := test.failure != "" && test.failure != "modern_absent" && test.failure != "modern_invalid" && !test.override
				if wantError {
					assert.Error(t, err, "a failed declared log key must not silently fall back to modern keys")
					assert.Nil(t, keys)
				} else {
					require.NoError(t, err)
					id := fixture.legacyID
					if test.modernOnly {
						id = fixture.modernID
					}
					require.Contains(t, keys.Keys, id)
					if !test.untagged && !test.modernOnly && !test.override && log.name == "rekor.pub" {
						require.Equal(t, legacytuf.Expired, keys.Keys[id].Status, "tagged key status must survive")
					}
				}
				material, err := VerificationMaterialFor(context.Background(), "", nil)
				if wantError || test.failure == "modern_invalid" {
					require.Error(t, err)
					require.Nil(t, material)
					return
				}
				require.NoError(t, err)
				materialKeys := material.RekorPublicKeys
				if log.name == "ctfe.pub" {
					materialKeys = material.CTLogPublicKeys
				}
				require.Equal(t, keyStatuses(keys.Keys), keyStatuses(materialKeys.Keys))
			})
		}
	}
}
