package sigstoreguard

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	legacytuf "github.com/sigstore/sigstore/pkg/tuf"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

func TestAuthenticatedTargetAbsenceWithDelegations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		mode   string
		absent bool
	}{
		{mode: "delegated", absent: true},
		{mode: "succinct", absent: true},
		{mode: "terminating", absent: true},
		{mode: "cycle", absent: true},
		{mode: "missing_metadata"},
		{mode: "invalid_metadata"},
		{mode: "traversal_exhausted"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			t.Parallel()
			fixture := newVerificationFixture(t)
			rootBytes, mirror, httpClient, files := fixture.delegatedRepository(t, test.mode)
			opts, err := tufOptions(context.Background(), mirror, rootBytes, false, httpClient, true)
			require.NoError(t, err)
			client, err := newUpdaterClient(opts)
			require.NoError(t, err)
			require.False(t, authenticatedTargetAbsent(client, "rekor.pub"), "unloaded matching roles do not prove absence")
			_, err = client.GetTarget("rekor.pub")
			require.Error(t, err)
			require.Equal(t, test.absent, authenticatedTargetAbsent(client, "rekor.pub"))
			if !test.absent {
				return
			}
			// A signed declaration remains present when its target download
			// fails; it must never be mistaken for an optional absent file.
			for path := range files {
				if strings.HasPrefix(path, "/targets/") {
					delete(files, path)
				}
			}
			_, err = client.GetTarget("trusted_root.json")
			require.Error(t, err)
			require.False(t, authenticatedTargetAbsent(client, "trusted_root.json"))
		})
	}
}

func TestVerificationMaterialWithModernDelegatedRepository(t *testing.T) {
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	for _, mode := range []string{"delegated", "succinct", "terminating", "cycle"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newVerificationFixture(t)
			rootBytes, mirror, client, _ := fixture.delegatedRepository(t, mode)
			// Each legacy fallback name matches a delegation, but completed
			// authenticated lookups establish that only modern trust exists.
			material, err := verificationMaterialFor(context.Background(), mirror, rootBytes, client)
			require.NoError(t, err)
			require.NotNil(t, material.TrustedRoot)
			require.Equal(t, map[string]legacytuf.StatusKind{fixture.modernID: legacytuf.Active}, keyStatuses(material.RekorPublicKeys.Keys))
			require.Equal(t, map[string]legacytuf.StatusKind{fixture.modernID: legacytuf.Active}, keyStatuses(material.CTLogPublicKeys.Keys))
			_, err = fixture.modernCA.Verify(x509.VerifyOptions{Roots: material.FulcioRoots, Intermediates: material.FulcioIntermediates})
			require.NoError(t, err)
		})
	}
}

func (f *verificationFixture) delegatedRepository(t *testing.T, mode string) ([]byte, string, *http.Client, map[string][]byte) {
	t.Helper()
	expires := time.Now().Add(time.Hour)
	signer, err := signature.LoadSignerVerifier(f.key, crypto.SHA256)
	require.NoError(t, err)
	key, err := metadata.KeyFromPublicKey(&f.key.PublicKey)
	require.NoError(t, err)
	keyID, err := key.ID()
	require.NoError(t, err)
	rootMetadata := metadata.Root(expires)
	for _, role := range []string{metadata.ROOT, metadata.TARGETS, metadata.SNAPSHOT, metadata.TIMESTAMP} {
		require.NoError(t, rootMetadata.Signed.AddKey(key, role))
	}
	roles := map[string]*metadata.Metadata[metadata.TargetsType]{metadata.TARGETS: metadata.Targets(expires)}
	delegate := func(parent, child string, terminating bool) {
		if roles[child] == nil {
			roles[child] = metadata.Targets(expires)
		}
		if roles[parent].Signed.Delegations == nil {
			roles[parent].Signed.Delegations = &metadata.Delegations{Keys: map[string]*metadata.Key{keyID: key}}
		}
		delegations := roles[parent].Signed.Delegations
		delegations.Roles = append(delegations.Roles, metadata.DelegatedRole{Name: child, KeyIDs: []string{keyID}, Threshold: 1, Paths: []string{"*"}, Terminating: terminating})
	}
	owner := "trust"
	switch mode {
	case "succinct":
		succinct := &metadata.SuccinctRoles{KeyIDs: []string{keyID}, Threshold: 1, BitLength: 1, NamePrefix: "bin"}
		roles[metadata.TARGETS].Signed.Delegations = &metadata.Delegations{Keys: map[string]*metadata.Key{keyID: key}, SuccinctRoles: succinct}
		for _, role := range succinct.GetRoles() {
			roles[role] = metadata.Targets(expires)
		}
		owner = succinct.GetRolesForTarget("trusted_root.json")[0].Name
	case "terminating":
		delegate(metadata.TARGETS, "first", false)
		delegate(metadata.TARGETS, "ignored", false)
		delegate("first", owner, true)
	case "cycle":
		delegate(metadata.TARGETS, "first", false)
		delegate("first", owner, false)
		delegate(owner, "first", false)
	case "traversal_exhausted":
		parent := metadata.TARGETS
		for i := range 40 {
			child := fmt.Sprintf("role-%d", i)
			delegate(parent, child, false)
			parent = child
		}
		owner = parent
	default:
		delegate(metadata.TARGETS, owner, false)
	}
	data, err := f.trustedRoot.MarshalJSON()
	require.NoError(t, err)
	target, err := metadata.TargetFile().FromBytes("trusted_root.json", data, "sha256")
	require.NoError(t, err)
	roles[owner].Signed.Targets["trusted_root.json"] = target
	files := map[string][]byte{"/targets/" + hex.EncodeToString(target.Hashes["sha256"]) + ".trusted_root.json": data}
	snapshot := metadata.Snapshot(expires)
	for name, role := range roles {
		_, err := role.Sign(signer)
		require.NoError(t, err)
		files["/1."+name+".json"], err = role.ToBytes(false)
		require.NoError(t, err)
		snapshot.Signed.Meta[name+".json"] = metadata.MetaFile(1)
	}
	type signedMetadata interface {
		Sign(signature.Signer) (*metadata.Signature, error)
		ToBytes(bool) ([]byte, error)
	}
	for path, document := range map[string]signedMetadata{"/1.root.json": rootMetadata, "/1.snapshot.json": snapshot, "/timestamp.json": metadata.Timestamp(expires)} {
		_, err := document.Sign(signer)
		require.NoError(t, err)
		files[path], err = document.ToBytes(false)
		require.NoError(t, err)
	}
	switch mode {
	case "missing_metadata":
		delete(files, "/1.trust.json")
	case "invalid_metadata":
		files["/1.trust.json"] = []byte(`{"signed":{},"signatures":[]}`)
	case "terminating":
		// A terminating descendant prevents backtracking to this sibling;
		// its absence must not make an otherwise complete lookup fail.
		delete(files, "/1.ignored.json")
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		data, ok := files[request.URL.Path]
		status := http.StatusOK
		if !ok {
			status = http.StatusNotFound
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header), Request: request}, nil
	})}
	return files["/1.root.json"], "https://delegated.example", client, files
}
