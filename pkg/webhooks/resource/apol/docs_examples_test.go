package apol

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/apis/apiserver/load"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	authorizationcel "k8s.io/apiserver/pkg/authorization/cel"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/clientcmd"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/yaml"
)

const examplesDir = "../../../../docs/dev/conditional-authorization/examples"

// TestDocsAuthorizationConfiguration validates the documented
// AuthorizationConfiguration with the kube-apiserver v0.38.0-beta.0 loader and
// validation, with and without the ConditionalAuthorization gate.
//
//nolint:paralleltest // mutates the global feature gate
func TestDocsAuthorizationConfiguration(t *testing.T) {
	cfg, err := load.LoadFromFile(filepath.Join(examplesDir, "authorization-config.yaml"))
	require.NoError(t, err)
	// The documented path only exists on a control-plane node; point it at the example kubeconfig.
	kubeconfig, err := filepath.Abs(filepath.Join(examplesDir, "kyverno-authz-kubeconfig.yaml"))
	require.NoError(t, err)
	cfg.Authorizers[2].Webhook.ConnectionInfo.KubeConfigFile = &kubeconfig
	known := sets.New("Node", "RBAC", "Webhook", "ABAC", "AlwaysAllow", "AlwaysDeny")
	repeatable := sets.New("Webhook")

	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, true)
	errs := apiservervalidation.ValidateAuthorizationConfiguration(authorizationcel.NewDefaultCompiler(), field.NewPath(""), cfg, known, repeatable)
	require.Empty(t, errs)
	require.Equal(t, "v1alpha1", cfg.Authorizers[2].Webhook.ConditionsReview.Version)

	featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, false)
	errs = apiservervalidation.ValidateAuthorizationConfiguration(authorizationcel.NewDefaultCompiler(), field.NewPath(""), cfg, known, repeatable)
	require.NotEmpty(t, errs, "conditionsReview must be rejected without the feature gate")
}

func TestDocsKubeconfig(t *testing.T) {
	t.Parallel()
	cfg, err := clientcmd.LoadFromFile(filepath.Join(examplesDir, "kyverno-authz-kubeconfig.yaml"))
	require.NoError(t, err)
	require.Contains(t, cfg.Contexts, "kyverno-conditions")
	require.Equal(t, "https://kyverno-svc.kyverno.svc:443/authz/conditionsreview", cfg.Clusters[cfg.Contexts["kyverno-conditions"].Cluster].Server)
	require.Equal(t, "https://kyverno-svc.kyverno.svc:443/authz/subjectaccessreview", cfg.Clusters[cfg.Contexts[cfg.CurrentContext].Cluster].Server)
}

func TestDocsExamplePolicy(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(examplesDir, "owner-writes.yaml"))
	require.NoError(t, err)
	var pol policiesv1alpha1.AuthorizingPolicy
	require.NoError(t, yaml.UnmarshalStrict(raw, &pol))
	_, errs := apolcompiler.NewCompiler().Compile(&pol)
	require.Empty(t, errs)
	h := newHandler(t, &pol)
	spec := func(verb string) *authorizationv1.SubjectAccessReviewSpec {
		return &authorizationv1.SubjectAccessReviewSpec{
			User: "alice", Groups: []string{"dev"},
			ResourceAttributes: &authorizationv1.ResourceAttributes{Namespace: "a", Verb: verb, Version: "v1", Resource: "configmaps"},
			AuthorizationOptions: &authorizationv1.AuthorizationOptions{
				HandledDecisionTypes: authorizationv1.ConditionalAuthorizationDecisionTypesList(),
			},
		}
	}
	require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeAllow, h.engine.Authorize(context.Background(), spec("list"), true).Type)
	require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeConditionsMap, h.engine.Authorize(context.Background(), spec("update"), true).Type)
}
