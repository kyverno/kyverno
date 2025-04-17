package internal

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/sigstore/cosign/v2/pkg/blob"
	"github.com/sigstore/sigstore/pkg/tuf"
)

func setupSigstoreTUF(ctx context.Context, logger logr.Logger) {
	if !enableTUF {
		return
	}

	logger = logger.WithName("sigstore-tuf").WithValues("tufRoot", tufRoot, "tufRootRaw", tufRootRaw, "tufMirror", tufMirror)
	logger.V(2).Info("setup tuf client for sigstore...")
	var tufRootBytes []byte
	var err error
	if tufRoot != "" {
		tufRootBytes, err = blob.LoadFileOrURL(tufRoot)
		if err != nil {
			checkError(logger, err, fmt.Sprintf("Failed to read alternate TUF root file %s : %v", tufRoot, err))
		}
	} else if tufRootRaw != "" {
		root, err := base64.StdEncoding.DecodeString(tufRootRaw)
		if err != nil {
			checkError(logger, err, fmt.Sprintf("Failed to base64 decode TUF root  %s : %v", tufRootRaw, err))
		}
		tufRootBytes = root
	}

	logger.V(2).Info("Initializing TUF root")
	if err := tuf.Initialize(ctx, tufMirror, tufRootBytes); err != nil {
		checkError(logger, err, fmt.Sprintf("Failed to initialize TUF client from %s : %v", tufRoot, err))
	}
}
