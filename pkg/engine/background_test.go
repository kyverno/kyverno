package engine

import (
	"context"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyBackgroundChecks_ContextPropagation(t *testing.T) {
	var got context.Context
	cfg := config.NewDefaultConfiguration(false)
	e := NewEngine(cfg, jmespath.New(cfg), nil, nil, imageverifycache.DisabledImageVerifyCache(),
		func(kyverno.PolicyInterface, kyverno.Rule) engineapi.ContextLoader {
			return loaderFunc(func(ctx context.Context) error { got = ctx; return nil })
		}, nil, nil)

	ctx := context.WithValue(context.Background(), "k", "v")
	policy := &kyverno.ClusterPolicy{Spec: kyverno.Spec{Rules: []kyverno.Rule{{
		Name:           "r",
		MatchResources: kyverno.MatchResources{ResourceDescription: kyverno.ResourceDescription{Kinds: []string{"ConfigMap"}}},
		Context:        []kyverno.ContextEntry{{Name: "x", APICall: &kyverno.ContextAPICall{}}},
		Generation:     &kyverno.Generation{Synchronize: true},
	}}}}
	var res unstructured.Unstructured
	res.SetAPIVersion("v1")
	res.SetKind("ConfigMap")
	res.SetName("test")
	res.SetNamespace("default")
	pCtx, _ := NewPolicyContext(jmespath.New(cfg), res, kyverno.Create, nil, cfg)

	e.ApplyBackgroundChecks(ctx, pCtx.WithPolicy(policy))

	assert.Equal(t, "v", got.Value("k"))
}

type loaderFunc func(context.Context) error

func (f loaderFunc) Load(ctx context.Context, _ jmespath.Interface, _ engineapi.RawClient, _ engineapi.RegistryClientFactory, _ []kyverno.ContextEntry, _ enginecontext.Interface) error {
	return f(ctx)
}
