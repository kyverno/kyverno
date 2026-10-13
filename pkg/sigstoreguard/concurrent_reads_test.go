package sigstoreguard

import (
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/stretchr/testify/require"
)

func TestConcurrentDefaultTUFReads(t *testing.T) {
	// Keep the SDK transport and operator defaults isolated from other tests.
	// A race-instrumented test binary also instruments this child process.
	const child = "KYVERNO_TEST_CONCURRENT_TUF_READS"
	if os.Getenv(child) != "true" {
		command := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestConcurrentDefaultTUFReads$", "-test.timeout=1m")
		command.Env = append(os.Environ(), child+"=true")
		output, err := command.CombinedOutput()
		require.NoError(t, err, "%s", output)
		return
	}
	t.Setenv("TUF_ROOT", t.TempDir())
	t.Setenv("SIGSTORE_NO_CACHE", "false")
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	initial, rotated, _, target, fixture := rotatingRepository(t)
	var metadataRequests, targetRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/targets/") {
			targetRequests.Add(1)
		} else {
			metadataRequests.Add(1)
		}
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
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "93.184.216.34:80" {
			return nil, fmt.Errorf("unexpected test destination %q", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}}
	t.Cleanup(transport.CloseIdleConnections)
	previous := regcreds.DefaultTransport
	regcreds.DefaultTransport = transport
	t.Cleanup(func() { regcreds.DefaultTransport = previous })
	const mirror = "http://93.184.216.34"
	SetDefaultRepository(mirror, initial)
	expected, err := root.NewTrustedRootFromJSON(target)
	require.NoError(t, err)
	var keyID string
	for id := range expected.RekorLogs() {
		keyID = id
	}
	require.NotEmpty(t, keyID)
	certificate := expected.FulcioCertificateAuthorities()[0].(*root.FulcioCertificateAuthority).Root
	checkRoot := func(tr *root.TrustedRoot) error {
		if tr == nil || tr.RekorLogs()[keyID] == nil || tr.CTLogs()[keyID] == nil {
			return fmt.Errorf("trust read did not return the fixture's authenticated log keys")
		}
		return nil
	}
	checkCertificate := func(roots *x509.CertPool) error {
		if roots == nil {
			return fmt.Errorf("trust read returned no Fulcio roots")
		}
		_, err := certificate.Verify(x509.VerifyOptions{Roots: roots})
		return err
	}
	operations := []struct {
		name string
		read func(context.Context) error
	}{
		{"initialize", func(ctx context.Context) error { return Initialize(ctx, mirror, initial) }},
		{"trusted root", func(ctx context.Context) error {
			tr, err := TrustedRoot(ctx)
			if err != nil {
				return err
			}
			return checkRoot(tr)
		}},
		{"verification material", func(ctx context.Context) error {
			material, err := VerificationMaterialFor(ctx, "", nil)
			if err != nil {
				return err
			}
			if err := checkRoot(material.TrustedRoot); err != nil {
				return err
			}
			return checkCertificate(material.FulcioRoots)
		}},
		{"Rekor keys", func(ctx context.Context) error {
			keys, err := RekorPublicKeys(ctx)
			if err == nil && (keys == nil || keys.Keys[keyID].PubKey == nil) {
				return fmt.Errorf("missing authenticated Rekor key")
			}
			return err
		}},
		{"CT log keys", func(ctx context.Context) error {
			keys, err := CTLogPublicKeys(ctx)
			if err == nil && (keys == nil || keys.Keys[keyID].PubKey == nil) {
				return fmt.Errorf("missing authenticated CT log key")
			}
			return err
		}},
		{"Fulcio roots", func(ctx context.Context) error {
			roots, _, err := FulcioRootsWithContext(ctx)
			if err != nil {
				return err
			}
			return checkCertificate(roots)
		}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := make(chan struct{})
	results := make(chan error, len(operations)*2)
	for range 2 {
		for _, operation := range operations {
			go func() {
				<-start
				for range 4 {
					if err := operation.read(ctx); err != nil {
						results <- fmt.Errorf("%s: %w", operation.name, err)
						return
					}
				}
				results <- nil
			}()
		}
	}
	close(start)
	for range len(operations) * 2 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	require.NoError(t, ctx.Err(), "the concurrent reads must complete with an uncanceled context")
	require.Positive(t, metadataRequests.Load(), "the test must authenticate real metadata")
	require.Positive(t, targetRequests.Load(), "the test must fetch real trust targets")
	retainedMirror, retainedRoot := defaultRepository()
	require.Equal(t, mirror, retainedMirror)
	require.JSONEq(t, string(rotated), string(retainedRoot))
}
