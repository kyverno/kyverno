package engine

import (
	"context"
	"os"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	pkgyaml "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	pssPolicies    []kyvernov1.PolicyInterface
	pssAdmitPod    unstructured.Unstructured
	pssDenyPod     unstructured.Unstructured
	pssBenchEngine engineapi.Engine
)

func init() {
	data, err := os.ReadFile("testdata/pss-restricted-clusterpolicies.yaml")
	if err != nil {
		panic(err)
	}
	policies, _, _, _, _, _, _, err := pkgyaml.GetPolicy(data)
	if err != nil {
		panic(err)
	}
	pssPolicies = policies

	admitJSON := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "pss-admit", "namespace": "default"},
		"spec": {
			"securityContext": {
				"runAsNonRoot": true,
				"seccompProfile": {"type": "RuntimeDefault"}
			},
			"containers": [{
				"name": "app",
				"image": "registry.example.io/app:1.0.0",
				"securityContext": {
					"allowPrivilegeEscalation": false,
					"runAsNonRoot": true,
					"capabilities": {"drop": ["ALL"]},
					"seccompProfile": {"type": "RuntimeDefault"}
				}
			}]
		}
	}`)
	denyJSON := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "pss-deny", "namespace": "default"},
		"spec": {
			"containers": [{
				"name": "app",
				"image": "registry.example.io/app:1.0.0",
				"securityContext": {"privileged": true}
			}]
		}
	}`)
	admit, err := kubeutils.BytesToUnstructured(admitJSON)
	if err != nil {
		panic(err)
	}
	deny, err := kubeutils.BytesToUnstructured(denyJSON)
	if err != nil {
		panic(err)
	}
	pssAdmitPod = *admit
	pssDenyPod = *deny

	cfg := config.NewDefaultConfiguration(false)
	pssBenchEngine = NewEngine(
		cfg,
		jp,
		nil,
		nil,
		imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil),
		nil,
		nil,
	)
}

func benchmarkPSSAdmission(b *testing.B, pod unstructured.Unstructured) {
	b.Helper()
	ctx := context.Background()
	pCtx, err := NewPolicyContext(jp, pod, kyvernov1.Create, nil, cfg)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, policy := range pssPolicies {
			pssBenchEngine.Validate(ctx, pCtx.WithPolicy(policy))
		}
	}
}

func BenchmarkValidatePSSAdmission_Admit(b *testing.B) {
	benchmarkPSSAdmission(b, pssAdmitPod)
}

func BenchmarkValidatePSSAdmission_Deny(b *testing.B) {
	benchmarkPSSAdmission(b, pssDenyPod)
}
