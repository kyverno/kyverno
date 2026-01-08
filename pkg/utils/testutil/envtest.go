//go:build envtest

package testutil

import (
	"path/filepath"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// EnvTestEnvironment wraps envtest.Environment with additional helpers
type EnvTestEnvironment struct {
	*envtest.Environment
	Config *rest.Config
	Client client.Client
	Scheme *runtime.Scheme
}

// SetupEnvTest initializes a test environment with real API server
// This should be used for integration tests that need controller reconciliation
func SetupEnvTest() (*EnvTestEnvironment, error) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crds", "policyreport"),
			filepath.Join("..", "..", "..", "config", "crds", "reports"),
			filepath.Join("..", "..", "..", "config", "crds", "kyverno"),
			filepath.Join("..", "..", "..", "config", "crds", "policies.kyverno.io"),
		},
		ErrorIfCRDPathMissing: true,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 60 * time.Second,
		},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		return nil, err
	}

	// Add Kyverno schemes
	s := scheme.Scheme
	_ = kyvernov1.AddToScheme(s)
	_ = kyvernov2.AddToScheme(s)

	k8sClient, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		testEnv.Stop()
		return nil, err
	}

	return &EnvTestEnvironment{
		Environment: testEnv,
		Config:      cfg,
		Client:      k8sClient,
		Scheme:      s,
	}, nil
}
