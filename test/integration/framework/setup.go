package framework

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// SetupEnvTest boots a lightweight local API server + etcd via envtest,
// installs all Kyverno CRDs, and returns the rest.Config for client creation.
func SetupEnvTest(t *testing.T) (*rest.Config, *runtime.Scheme) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kyvernov1.Install(scheme)
	_ = apiextensions.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			"../../../config/crds/kyverno",
		},
		Scheme: scheme,
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop envtest: %v", err)
		}
	})
	return cfg, scheme
}
