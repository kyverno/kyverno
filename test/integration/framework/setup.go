package framework

import (
	"context"
	"fmt"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// TestEnv wraps an envtest environment with a controller-runtime manager.
type TestEnv struct {
	Env             *envtest.Environment
	Mgr             ctrl.Manager
	Client          client.Client
	KubeClient      kubernetes.Interface
	Scheme          *kruntime.Scheme
	ContextProvider libs.Context
	cancel          context.CancelFunc
}

// NewTestEnv creates an envtest environment with Kyverno CEL policy CRDs installed.
// crdPaths should point to directories containing CRD YAML files (e.g. config/crds/policies.kyverno.io).
func NewTestEnv(crdPaths ...string) (*TestEnv, error) {
	scheme := kruntime.NewScheme()
	if err := policiesv1beta1.Install(scheme); err != nil {
		return nil, fmt.Errorf("failed to install policiesv1beta1 scheme: %w", err)
	}

	// Initialize the global reporting config to prevent nil dereference
	// in the handler's async audit goroutine.
	reportutils.NewReportingConfig(nil)

	env := &envtest.Environment{
		CRDDirectoryPaths: crdPaths,
		Scheme:            scheme,
	}

	cfg, err := env.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start envtest: %w", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: "0", // disable metrics server in tests
		},
	})
	if err != nil {
		_ = env.Stop()
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// Create real kubernetes and dynamic clients from envtest config,
	// mirroring the production wiring in cmd/kyverno/main.go.
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		_ = env.Stop()
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		_ = env.Stop()
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create dclient using real clients backed by envtest's API server.
	dc, err := dclient.NewClient(context.Background(), dynClient, kubeClient, 10*time.Minute, false, nil)
	if err != nil {
		_ = env.Stop()
		return nil, fmt.Errorf("failed to create dclient: %w", err)
	}

	// Create the real ContextProvider — same code path as production.
	// Only the underlying K8s API is swapped (envtest instead of real cluster).
	ctxProvider, err := libs.NewContextProvider(dc, nil, nil, mgr.GetRESTMapper(), false)
	if err != nil {
		_ = env.Stop()
		return nil, fmt.Errorf("failed to create context provider: %w", err)
	}

	return &TestEnv{
		Env:             env,
		Mgr:             mgr,
		Client:          mgr.GetClient(),
		KubeClient:      kubeClient,
		Scheme:          scheme,
		ContextProvider: ctxProvider,
	}, nil
}

// Start starts the manager in a background goroutine and waits for cache sync.
func (te *TestEnv) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	te.cancel = cancel

	go func() {
		if err := te.Mgr.Start(ctx); err != nil {
			panic(fmt.Sprintf("manager failed to start: %v", err))
		}
	}()

	if !te.Mgr.GetCache().WaitForCacheSync(ctx) {
		cancel()
		return fmt.Errorf("failed to sync manager cache")
	}
	return nil
}

// Stop stops the manager and envtest environment.
func (te *TestEnv) Stop() {
	if te.cancel != nil {
		te.cancel()
	}
	_ = te.Env.Stop()
}
