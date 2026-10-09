package sigstoreguard

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestIsolatedTUFDoesNotWaitForSharedCache(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(context.Context) error
	}{
		{"trusted_root", func(ctx context.Context) error {
			_, err := TrustedRootFor(ctx, "http://169.254.169.254", nil)
			return err
		}},
		{"verification_material", func(ctx context.Context) error {
			_, err := VerificationMaterialFor(ctx, "http://169.254.169.254", nil)
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			locked, release, unlocked := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() { defer close(unlocked); _ = WithLock(func() error { close(locked); <-release; return nil }) }()
			<-locked
			defer func() { close(release); <-unlocked }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- operation.call(ctx) }()
			select {
			case err := <-done:
				require.ErrorIs(t, err, egress.ErrAddressBlocked)
			case <-ctx.Done():
				t.Fatal("isolated repository waited for the shared cache lock")
			}
		})
	}
}

func TestSharedTUFWaitHonorsCancellation(t *testing.T) {
	previousMirror, previousRoot := defaultRepository()
	SetDefaultRepository("http://169.254.169.254", nil)
	defer SetDefaultRepository(previousMirror, previousRoot)
	t.Setenv("SIGSTORE_REKOR_PUBLIC_KEY", "")
	t.Setenv("SIGSTORE_CT_LOG_PUBLIC_KEY_FILE", "")
	locked, release, unlocked := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { defer close(unlocked); _ = WithLock(func() error { close(locked); <-release; return nil }) }()
	<-locked
	defer func() { close(release); <-unlocked }()
	for _, operation := range []struct {
		name string
		call func(context.Context) error
	}{
		{"initialize", func(ctx context.Context) error { return Initialize(ctx, "http://169.254.169.254", nil) }},
		{"trusted_root", func(ctx context.Context) error { _, err := TrustedRoot(ctx); return err }},
		{"verification_material", func(ctx context.Context) error { _, err := VerificationMaterialFor(ctx, "", nil); return err }},
		{"rekor", func(ctx context.Context) error { _, err := RekorPublicKeys(ctx); return err }},
		{"ct_log", func(ctx context.Context) error { _, err := CTLogPublicKeys(ctx); return err }},
		{"fulcio", func(ctx context.Context) error { _, _, err := FulcioRootsWithContext(ctx); return err }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- operation.call(ctx) }()
			select {
			case err := <-done:
				// Without serialization this immediately reaches the blocked mirror;
				// the shared reader must instead wait only until its deadline.
				require.ErrorIs(t, err, context.DeadlineExceeded)
			case <-time.After(2 * time.Second):
				t.Fatal("expired request waited for the shared cache lock")
			}
		})
	}
}

func TestSlowPolicyTUFDoesNotBlockOtherRepositories(t *testing.T) {
	mu.Lock()
	previousMirror, previousRoot := defaultMirror, append([]byte(nil), defaultRoot...)
	mu.Unlock()
	SetDefaultRepository("http://169.254.169.254", nil)
	defer SetDefaultRepository(previousMirror, previousRoot)
	ctx, cancel := context.WithCancel(context.Background())
	started, done := make(chan struct{}), make(chan error, 1)
	var once sync.Once
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		once.Do(func() { close(started) })
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	go func() { _, err := verificationMaterialFor(ctx, "https://policy.example", nil, client); done <- err }()
	defer func() { cancel(); <-done }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("policy request did not reach its isolated transport")
	}
	for _, isolated := range []bool{false, true} {
		t.Run(fmt.Sprintf("isolated=%t", isolated), func(t *testing.T) {
			completed := make(chan error, 1)
			go func() {
				mirror := ""
				if isolated {
					mirror = "http://169.254.169.254"
				}
				_, err := TrustedRootFor(context.Background(), mirror, nil)
				completed <- err
			}()
			select {
			case err := <-completed:
				require.ErrorIs(t, err, egress.ErrAddressBlocked)
			case <-time.After(2 * time.Second):
				t.Error("unrelated trust read blocked behind a policy-specific fetch")
			}
		})
	}
}
