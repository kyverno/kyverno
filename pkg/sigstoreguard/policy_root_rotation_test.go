package sigstoreguard

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/stretchr/testify/require"
)

func TestPolicyRootRotationDoesNotPublishOperatorRoot(t *testing.T) {
	// Registry transports and operator trust are process-wide. Give each public
	// API a fresh process so its guarded transport can dial only this fixture.
	const child = "KYVERNO_TEST_POLICY_ROOT_ROTATION"
	caller := os.Getenv(child)
	if caller == "" {
		for _, caller := range []string{"trusted_root", "verification_material"} {
			t.Run(caller, func(t *testing.T) {
				command := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestPolicyRootRotationDoesNotPublishOperatorRoot$", "-test.timeout=1m")
				command.Env = append(os.Environ(), child+"="+caller)
				output, err := command.CombinedOutput()
				require.NoError(t, err, "%s", output)
			})
		}
		return
	}

	cacheRoot := t.TempDir()
	t.Setenv("TUF_ROOT", cacheRoot)
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	initial, rotated, _, target, fixture := rotatingRepository(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := fixture.Transport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer response.Body.Close()
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(server.Close)
	// Keep the production egress checks, while connecting the validated public
	// test address only to the local signed repository.
	previous := regcreds.DefaultTransport
	regcreds.DefaultTransport = &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "93.184.216.34:80" {
			return nil, fmt.Errorf("unexpected test destination %q", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	t.Cleanup(func() { regcreds.DefaultTransport = previous })
	mirror := "http://93.184.216.34"
	SetDefaultRepository(mirror, initial)
	cachePath := filepath.Join(cacheRoot, tuf.URLToPath(mirror), "root.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o700))
	// A failed initialization can leave an authenticated rotation on disk
	// without publishing it as the operator's active root.
	require.NoError(t, os.WriteFile(cachePath, rotated, 0o600))

	read := func(mirror string, rootBytes []byte) ([]byte, error) {
		if caller == "trusted_root" {
			tr, err := TrustedRootFor(context.Background(), mirror, rootBytes)
			if err != nil {
				return nil, err
			}
			return tr.MarshalJSON()
		}
		material, err := VerificationMaterialFor(context.Background(), mirror, rootBytes)
		if err != nil {
			return nil, err
		}
		return material.TrustedRoot.MarshalJSON()
	}
	actual, err := read(mirror, initial)
	require.NoError(t, err)
	require.JSONEq(t, string(target), string(actual), "the policy must authenticate the new root before reading its signed target")
	retainedMirror, retainedRoot := defaultRepository()
	require.Equal(t, mirror, retainedMirror)
	require.JSONEq(t, string(initial), string(retainedRoot), "an explicit policy must not publish its rotation even when its configuration matches the operator")
	cached, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	require.Equal(t, rotated, cached, "the explicit policy must not overwrite the operator cache")

	actual, err = read("", nil)
	require.NoError(t, err)
	require.JSONEq(t, string(target), string(actual))
	retainedMirror, retainedRoot = defaultRepository()
	require.Equal(t, mirror, retainedMirror)
	require.JSONEq(t, string(rotated), string(retainedRoot), "an ordinary default read must still retain its authenticated rotation")
}
