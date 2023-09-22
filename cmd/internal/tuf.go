package internal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"github.com/sigstore/sigstore/pkg/tuf"
)

func setupSigstoreTUF(ctx context.Context, logger logr.Logger) {
	if !enableTUF {
		return
	}

	logger = logger.WithName("sigstore-tuf").WithValues("tufroot", tufRoot, "tufmirror", tufMirror)
	logger.Info("setup tuf client for sigstore...")
	var tufRootBytes []byte
	var err error
	if tufRoot != "" {
		tufRootBytes, err = blob.LoadFileOrURL(tufRoot)
		if err != nil {
			checkError(logger, err, fmt.Sprintf("Failed to read alternate TUF root file %s : %v", tufRoot, err))
		}
	}
	logger.Info("Initializing TUF root")
	if err := tuf.Initialize(ctx, tufMirror, tufRootBytes); err != nil {
		checkError(logger, err, fmt.Sprintf("Failed to initialize TUF client from %s : %v", tufRoot, err))
	}
}
