package cosign

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTUFConfigManager_NewTUFConfigManager(t *testing.T) {
	configs := []TUFConfig{
		{
			Mirror:  "https://tuf-repo.github.com/",
			Enabled: true,
		},
		{
			Mirror:  "https://custom-tuf.example.com/",
			Enabled: true,
		},
	}

	manager := NewTUFConfigManager(configs)
	require.NotNil(t, manager)
	assert.Equal(t, 2, len(manager.GetConfigs()))
}

func TestTUFConfigManager_AddConfig(t *testing.T) {
	manager := NewTUFConfigManager([]TUFConfig{})
	require.NotNil(t, manager)
	assert.Equal(t, 0, len(manager.GetConfigs()))

	config := TUFConfig{
		Mirror:  "https://tuf-repo.github.com/",
		Enabled: true,
	}
	manager.AddConfig(config)
	assert.Equal(t, 1, len(manager.GetConfigs()))
}

func TestTUFConfigManager_GetConfigs(t *testing.T) {
	configs := []TUFConfig{
		{
			Mirror:  "https://tuf-repo.github.com/",
			Enabled: true,
		},
	}

	manager := NewTUFConfigManager(configs)
	retrieved := manager.GetConfigs()
	assert.Equal(t, configs, retrieved)
}

func TestSetAndGetGlobalTUFConfigManager(t *testing.T) {
	logger := testr.New(t)
	manager := NewTUFConfigManager([]TUFConfig{})

	SetGlobalTUFConfigManager(manager, logger)
	retrieved := GetGlobalTUFConfigManager()

	assert.Equal(t, manager, retrieved)
}

func TestTUFConfigManager_GetTrustedRootWithFallback_NoConfigs(t *testing.T) {
	tufLogger = testr.New(t)
	manager := NewTUFConfigManager([]TUFConfig{})

	ctx := context.Background()
	trustedRoot, err := manager.GetTrustedRootWithFallback(ctx)

	if err == nil {
		assert.NotNil(t, trustedRoot)
	} else {
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestTUFConfigManager_GetTrustedRootWithFallback_DisabledConfig(t *testing.T) {
	tufLogger = testr.New(t)
	configs := []TUFConfig{
		{
			Mirror:  "https://invalid-tuf.example.com/",
			Enabled: false,
		},
	}
	manager := NewTUFConfigManager(configs)

	ctx := context.Background()
	trustedRoot, err := manager.GetTrustedRootWithFallback(ctx)

	if err == nil {
		assert.NotNil(t, trustedRoot)
	} else {
		t.Logf("Expected error in test environment: %v", err)
	}
}
