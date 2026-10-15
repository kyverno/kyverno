package sigstoreguard

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestFulcioRepositoryRejectsFailedDeclaredTargets(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	t.Setenv("SIGSTORE_NO_CACHE", "true")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	for _, api := range []string{"default_fulcio", "verification_material"} {
		for _, test := range []struct {
			name       string
			target     string
			failure    string
			untagged   bool
			modernOnly bool
		}{
			{name: "mixed"},
			{name: "modern_only", modernOnly: true},
			{name: "pem_only", target: "trusted_root.json", failure: "absent"},
			{name: "modern_missing", target: "trusted_root.json", failure: "download"},
			{name: "modern_hash", target: "trusted_root.json", failure: "hash"},
			{name: "modern_json", target: "trusted_root.json", failure: "malformed"},
			{name: "tagged_legacy_missing", target: "fulcio-previous.pem", failure: "download"},
			{name: "tagged_legacy_hash", target: "fulcio-previous.pem", failure: "hash"},
			{name: "tagged_legacy_pem", target: "fulcio-previous.pem", failure: "malformed"},
			{name: "tagged_legacy_empty", target: "fulcio-previous.pem", failure: "empty"},
			{name: "untagged_legacy_missing", target: "fulcio_v1.crt.pem", failure: "download", untagged: true},
			{name: "untagged_legacy_hash", target: "fulcio_v1.crt.pem", failure: "hash", untagged: true},
			{name: "untagged_legacy_pem", target: "fulcio_v1.crt.pem", failure: "malformed", untagged: true},
		} {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				t.Setenv("TUF_ROOT", t.TempDir())
				fixture := newVerificationFixture(t)
				var files map[string][]byte
				rootBytes, _, _ := fixture.repositoryWithChanges(t, !test.modernOnly, func(targets *metadata.Metadata[metadata.TargetsType], repository map[string][]byte) {
					files = repository
					if test.untagged {
						info := targets.Signed.Targets["fulcio-previous.pem"]
						data := files[fixtureTargetPath("fulcio-previous.pem", info)]
						delete(targets.Signed.Targets, "fulcio-previous.pem")
						delete(files, fixtureTargetPath("fulcio-previous.pem", info))
						// A second, valid untagged root must not hide a failed
						// declared sibling during fixed-name fallback.
						for _, name := range []string{"fulcio.crt.pem", "fulcio_v1.crt.pem"} {
							replacement, err := metadata.TargetFile().FromBytes(name, data, "sha256")
							require.NoError(t, err)
							targets.Signed.Targets[name] = replacement
							files[fixtureTargetPath(name, replacement)] = data
						}
					}
					if test.target == "" {
						return
					}
					info := targets.Signed.Targets[test.target]
					path := fixtureTargetPath(test.target, info)
					switch test.failure {
					case "absent":
						delete(targets.Signed.Targets, test.target)
						delete(files, path)
					case "download":
						delete(files, path)
					case "hash":
						files[path] = bytes.Repeat([]byte("x"), len(files[path]))
					case "malformed", "empty":
						data := []byte("invalid trust material")
						if test.failure == "empty" {
							data = nil
						}
						replacement, err := metadata.TargetFile().FromBytes(test.target, data, "sha256")
						require.NoError(t, err)
						replacement.Custom = info.Custom
						targets.Signed.Targets[test.target] = replacement
						delete(files, path)
						files[fixtureTargetPath(test.target, replacement)] = data
					}
				})
				// Exercise the real public APIs using a signed operator file
				// mirror, without replacing their process-wide HTTP clients.
				directory := t.TempDir()
				for name, data := range files {
					path := filepath.Join(directory, name)
					require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
					require.NoError(t, os.WriteFile(path, data, 0o600))
				}
				SetDefaultRepository("file://"+directory, rootBytes)
				var roots, intermediates *x509.CertPool
				var err error
				if api == "default_fulcio" {
					roots, intermediates, err = FulcioRootsWithContext(context.Background())
				} else {
					material, materialErr := VerificationMaterialFor(context.Background(), "", nil)
					err = materialErr
					if material != nil {
						roots, intermediates = material.FulcioRoots, material.FulcioIntermediates
					}
				}
				if test.failure != "" && test.failure != "absent" {
					require.Error(t, err, "failed declared trust material must not silently fall back to the other format")
					require.Nil(t, roots)
					require.Nil(t, intermediates)
					return
				}
				require.NoError(t, err)
				if test.failure != "absent" {
					_, err = fixture.modernCA.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates})
					require.NoError(t, err)
				}
				if !test.modernOnly {
					_, err = fixture.historicalLeaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, CurrentTime: fixture.signingTime, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}})
					require.NoError(t, err)
				}
			})
		}
	}
}

func fixtureTargetPath(name string, info *metadata.TargetFiles) string {
	return "/targets/" + hex.EncodeToString(info.Hashes["sha256"]) + "." + name
}
