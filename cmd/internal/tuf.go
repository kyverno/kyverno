package internal

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/sigstoreguard"
	"github.com/kyverno/kyverno/pkg/sigstoretuf"
)

func setupSigstoreTUF(ctx context.Context, logger logr.Logger) {
	if !enableTUF {
		return
	}

	logger = logger.WithName("sigstore-tuf").WithValues("tufRoot", tufRoot, "tufRootRaw", tufRootRaw, "tufMirror", tufMirror)
	logger.V(2).Info("setup tuf client for sigstore...")
	tufRootBytes, err := loadTUFRoot(ctx, tufRoot, tufRootRaw)
	if err != nil {
		checkError(logger, err, "Failed to load alternate TUF root")
	}

	// Preserve the existing singleton while the guarded verifier owns its repository.
	sigstoreguard.SetDefaultRepository(tufMirror, tufRootBytes)
	logger.V(2).Info("Initializing TUF root")
	if err := sigstoretuf.Initialize(ctx, tufMirror, tufRootBytes); err != nil {
		checkError(logger, err, fmt.Sprintf("Failed to initialize TUF client from %s : %v", tufRoot, err))
	}
}

// loadTUFRoot keeps file references ahead of inline roots and applies the
// configured registry destination checks to HTTP root downloads.
func loadTUFRoot(ctx context.Context, ref, raw string) ([]byte, error) {
	if ref != "" {
		data, err := sigstoreguard.LoadFileOrURL(ctx, ref)
		if err != nil {
			return nil, fmt.Errorf("reading alternate TUF root %s: %w", ref, err)
		}
		return data, nil
	}
	if raw != "" {
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("decoding alternate TUF root: %w", err)
		}
		return data, nil
	}
	return nil, nil
}
