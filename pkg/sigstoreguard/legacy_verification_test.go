package sigstoreguard

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestVerificationMaterialPEMOnlyOperatorRepository(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	t.Cleanup(func() { SetDefaultRepository(previousMirror, previousRoot) })
	t.Setenv("SIGSTORE_NO_CACHE", "true")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	for _, modern := range []string{"absent", "valid", "download_missing", "hash_mismatch", "malformed", "delegation_unavailable"} {
		t.Run(modern, func(t *testing.T) {
			t.Setenv("TUF_ROOT", t.TempDir())
			fixture := newVerificationFixture(t)
			rootBytes, mirror, client := fixture.repositoryWithChanges(t, true, func(targets *metadata.Metadata[metadata.TargetsType], files map[string][]byte) {
				name := "trusted_root.json"
				info := targets.Signed.Targets[name]
				path := "/targets/" + hex.EncodeToString(info.Hashes["sha256"]) + "." + name
				switch modern {
				case "absent", "delegation_unavailable":
					delete(targets.Signed.Targets, name)
					delete(files, path)
					if modern == "delegation_unavailable" {
						key, err := metadata.KeyFromPublicKey(&fixture.key.PublicKey)
						require.NoError(t, err)
						keyID, err := key.ID()
						require.NoError(t, err)
						targets.Signed.Delegations = &metadata.Delegations{
							Keys:  map[string]*metadata.Key{keyID: key},
							Roles: []metadata.DelegatedRole{{Name: "modern", KeyIDs: []string{keyID}, Threshold: 1, Paths: []string{name}}},
						}
					}
				case "download_missing":
					delete(files, path)
				case "hash_mismatch":
					files[path] = bytes.Repeat([]byte("x"), len(files[path]))
				case "malformed":
					data := []byte("not a trusted root")
					replacement, err := metadata.TargetFile().FromBytes(name, data, "sha256")
					require.NoError(t, err)
					targets.Signed.Targets[name] = replacement
					delete(files, path)
					files["/targets/"+hex.EncodeToString(replacement.Hashes["sha256"])+"."+name] = data
				}
			})
			SetDefaultRepository(mirror, rootBytes)
			material, err := verificationMaterialFor(context.Background(), "", nil, client)
			if modern != "absent" && modern != "valid" {
				require.Error(t, err, "a failed modern target must not downgrade to the valid legacy targets")
				require.Nil(t, material)
				return
			}
			require.NoError(t, err)
			if modern == "absent" {
				require.Nil(t, material.TrustedRoot)
			} else {
				require.NotNil(t, material.TrustedRoot)
			}
			require.Equal(t, map[string]legacytuf.StatusKind{fixture.legacyID: legacytuf.Expired}, keyStatuses(material.RekorPublicKeys.Keys))
			require.Equal(t, map[string]legacytuf.StatusKind{fixture.legacyID: legacytuf.Active}, keyStatuses(material.CTLogPublicKeys.Keys))
			_, err = fixture.historicalLeaf.Verify(x509.VerifyOptions{Roots: material.FulcioRoots, Intermediates: material.FulcioIntermediates, CurrentTime: fixture.signingTime, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}})
			require.NoError(t, err)
			if modern == "absent" {
				_, err = verificationMaterialFor(context.Background(), mirror, rootBytes, client)
				require.Error(t, err, "an explicit policy repository still requires modern trust material")
				for _, environment := range []string{"SIGSTORE_REKOR_PUBLIC_KEY", "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE"} {
					t.Run(environment, func(t *testing.T) {
						file := filepath.Join(t.TempDir(), "operator.pub")
						key, err := publicKeyToPEM(&fixture.key.PublicKey)
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(file, key, 0o600))
						t.Setenv(environment, file)
						material, err := verificationMaterialFor(context.Background(), "", nil, client)
						require.NoError(t, err)
						keys := material.RekorPublicKeys
						if environment == "SIGSTORE_CT_LOG_PUBLIC_KEY_FILE" {
							keys = material.CTLogPublicKeys
						}
						require.Contains(t, keys.Keys, fixture.modernID)
						t.Setenv(environment, file+"-missing")
						material, err = verificationMaterialFor(context.Background(), "", nil, client)
						require.ErrorIs(t, err, os.ErrNotExist)
						require.Nil(t, material)
					})
				}
			}
		})
	}
}
