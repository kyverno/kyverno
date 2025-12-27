package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// TestEnvironment wraps envtest.Environment with additional utilities
type TestEnvironment struct {
	*envtest.Environment
	Config *rest.Config
	Client client.Client
	cancel context.CancelFunc
}

// SetupEnvTest initializes a test environment with Kyverno CRDs
func SetupEnvTest() (*TestEnvironment, error) {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	_, cancel := context.WithCancel(context.Background())

	// Find CRD path
	crdPath := getCRDPath()

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(crdPath, "kyverno"),
			filepath.Join(crdPath, "policies.kyverno.io"),
			filepath.Join(crdPath, "policyreport"),
			filepath.Join(crdPath, "reports"),
		},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			CleanUpAfterUse: true,
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start test environment: %w", err)
	}

	// Register Kyverno schemes
	if err := kyvernov1.AddToScheme(scheme.Scheme); err != nil {
		cancel()
		_ = testEnv.Stop()
		return nil, fmt.Errorf("failed to register kyverno v1 scheme: %w", err)
	}

	if err := kyvernov2.AddToScheme(scheme.Scheme); err != nil {
		cancel()
		_ = testEnv.Stop()
		return nil, fmt.Errorf("failed to register kyverno v2 scheme: %w", err)
	}

	// Create client
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		cancel()
		_ = testEnv.Stop()
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &TestEnvironment{
		Environment: testEnv,
		Config:      cfg,
		Client:      k8sClient,
		cancel:      cancel,
	}, nil
}

// Stop tears down the test environment
func (te *TestEnvironment) Stop() error {
	te.cancel()
	return te.Environment.Stop()
}

// getCRDPath finds the CRD directory
func getCRDPath() string {
	// Try common paths
	paths := []string{
		filepath.Join("config", "crds"),
		filepath.Join("..", "..", "config", "crds"),
		filepath.Join("..", "..", "..", "config", "crds"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fallback to environment variable
	if path := os.Getenv("KYVERNO_CRD_PATH"); path != "" {
		return path
	}

	return "config/crds"
}

// WaitForCondition waits for a condition to be met with timeout
func WaitForCondition(parent context.Context, timeout time.Duration, condition func() bool) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition")
		case <-ticker.C:
			if condition() {
				return nil
			}
		}
	}
}
