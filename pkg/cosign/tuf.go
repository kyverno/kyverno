package cosign

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/tuf"
)

var (
	tufConfigManager *TUFConfigManager
	tufManagerMu     sync.RWMutex
	tufLogger        logr.Logger
)

type TUFConfig struct {
	Mirror    string
	RootBytes []byte
	Enabled   bool
}

type TUFConfigManager struct {
	configs []TUFConfig
	mu      sync.RWMutex
}

func NewTUFConfigManager(configs []TUFConfig) *TUFConfigManager {
	return &TUFConfigManager{
		configs: configs,
	}
}

func SetGlobalTUFConfigManager(manager *TUFConfigManager, log logr.Logger) {
	tufManagerMu.Lock()
	defer tufManagerMu.Unlock()
	tufConfigManager = manager
	tufLogger = log.WithName("tuf-manager")
}

func GetGlobalTUFConfigManager() *TUFConfigManager {
	tufManagerMu.RLock()
	defer tufManagerMu.RUnlock()
	return tufConfigManager
}

func (m *TUFConfigManager) AddConfig(config TUFConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = append(m.configs, config)
}

func (m *TUFConfigManager) GetConfigs() []TUFConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configs
}

func (m *TUFConfigManager) GetTrustedRootWithFallback(ctx context.Context) (*root.TrustedRoot, error) {
	m.mu.RLock()
	configs := m.configs
	m.mu.RUnlock()

	var lastErr error

	for i, config := range configs {
		if !config.Enabled {
			continue
		}

		tufLogger.V(3).Info("attempting to get trusted root from TUF", "index", i, "mirror", config.Mirror)

		trustedRoot, err := getTrustedRootFromTUFConfig(ctx, config)
		if err == nil {
			tufLogger.V(2).Info("successfully retrieved trusted root from TUF", "index", i, "mirror", config.Mirror)
			return trustedRoot, nil
		}

		tufLogger.V(3).Info("failed to get trusted root from TUF", "index", i, "mirror", config.Mirror, "error", err.Error())
		lastErr = err
	}

	tufLogger.V(2).Info("attempting fallback to public Sigstore TUF")
	trustedRoot, err := getTrustedRootFromPublicTUF(ctx)
	if err == nil {
		tufLogger.V(2).Info("successfully retrieved trusted root from public Sigstore TUF")
		return trustedRoot, nil
	}

	tufLogger.V(2).Info("failed to get trusted root from public Sigstore TUF", "error", err.Error())

	if lastErr != nil {
		return nil, fmt.Errorf("failed to get trusted root from all TUF sources, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("failed to get trusted root from public Sigstore TUF: %w", err)
}

func getTrustedRootFromTUFConfig(ctx context.Context, config TUFConfig) (*root.TrustedRoot, error) {
	if err := tuf.Initialize(ctx, config.Mirror, config.RootBytes); err != nil {
		return nil, fmt.Errorf("initializing tuf with mirror %s: %w", config.Mirror, err)
	}

	tufClient, err := tuf.NewFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating tuf client: %w", err)
	}

	targetBytes, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json: %w", err)
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(targetBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}

	return trustedRoot, nil
}

func getTrustedRootFromPublicTUF(ctx context.Context) (*root.TrustedRoot, error) {
	if err := tuf.Initialize(ctx, tuf.DefaultRemoteRoot, nil); err != nil {
		return nil, fmt.Errorf("initializing public tuf: %w", err)
	}

	tufClient, err := tuf.NewFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating tuf client: %w", err)
	}

	targetBytes, err := tufClient.GetTarget("trusted_root.json")
	if err != nil {
		return nil, fmt.Errorf("getting trusted_root.json: %w", err)
	}

	trustedRoot, err := root.NewTrustedRootFromJSON(targetBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing trusted root: %w", err)
	}

	return trustedRoot, nil
}
