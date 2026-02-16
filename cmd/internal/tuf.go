package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/blob"
	"github.com/sigstore/sigstore/pkg/tuf"
)

func setupSigstoreTUF(ctx context.Context, logger logr.Logger) {
	logger = logger.WithName("sigstore-tuf")

	configs := []cosign.TUFConfig{}

	if tufConfigsJSON := os.Getenv("TUF_CONFIGS"); tufConfigsJSON != "" {
		logger.V(2).Info("loading TUF configurations from TUF_CONFIGS environment variable")
		var envConfigs []map[string]string
		if err := json.Unmarshal([]byte(tufConfigsJSON), &envConfigs); err != nil {
			logger.Error(err, "failed to parse TUF_CONFIGS, ignoring", "value", tufConfigsJSON)
		} else {
			for i, cfg := range envConfigs {
				mirror := cfg["mirror"]
				if mirror == "" {
					mirror = tuf.DefaultRemoteRoot
				}

				var rootBytes []byte
				var err error

				if rootPath := cfg["root"]; rootPath != "" {
					rootBytes, err = blob.LoadFileOrURL(rootPath)
					if err != nil {
						logger.Error(err, "failed to load TUF root from path, skipping config", "index", i, "path", rootPath)
						continue
					}
				} else if rootRaw := cfg["rootRaw"]; rootRaw != "" {
					rootBytes, err = base64.StdEncoding.DecodeString(rootRaw)
					if err != nil {
						logger.Error(err, "failed to decode TUF rootRaw, skipping config", "index", i)
						continue
					}
				}

				configs = append(configs, cosign.TUFConfig{
					Mirror:    mirror,
					RootBytes: rootBytes,
					Enabled:   true,
				})
				logger.V(2).Info("added TUF configuration from TUF_CONFIGS", "index", i, "mirror", mirror)
			}
		}
	}

	if enableTUF {
		logger = logger.WithValues("tufRoot", tufRoot, "tufRootRaw", tufRootRaw, "tufMirror", tufMirror)
		logger.V(2).Info("setup tuf client for sigstore from flags...")

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

		if tufMirror != "" || tufRootBytes != nil {
			mirror := tufMirror
			if mirror == "" {
				mirror = tuf.DefaultRemoteRoot
			}

			configs = append(configs, cosign.TUFConfig{
				Mirror:    mirror,
				RootBytes: tufRootBytes,
				Enabled:   true,
			})
			logger.V(2).Info("Added custom TUF configuration from flags", "mirror", mirror)
		}
	}

	if tufMirrorsStr := os.Getenv("TUF_MIRRORS"); tufMirrorsStr != "" && strings.Contains(tufMirrorsStr, ",") {
		mirrors := strings.Split(tufMirrorsStr, ",")
		logger.V(2).Info("loading TUF configurations from TUF_MIRRORS", "count", len(mirrors))
		for i, mirror := range mirrors {
			mirror = strings.TrimSpace(mirror)
			if mirror == "" {
				continue
			}
			configs = append(configs, cosign.TUFConfig{
				Mirror:    mirror,
				RootBytes: nil,
				Enabled:   true,
			})
			logger.V(2).Info("added TUF configuration from TUF_MIRRORS", "index", i, "mirror", mirror)
		}
	}

	manager := cosign.NewTUFConfigManager(configs)
	cosign.SetGlobalTUFConfigManager(manager, logger)

	if len(configs) == 0 {
		logger.V(2).Info("No custom TUF configurations, will fallback to public Sigstore TUF")
	} else {
		logger.V(2).Info("TUF configuration manager initialized with fallback to public Sigstore", "configCount", len(configs))
	}
}
