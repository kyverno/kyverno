package apol

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	crdcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	crdvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/apis/apiserver/load"
	apiservervalidation "k8s.io/apiserver/pkg/apis/apiserver/validation"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	authorizationcel "k8s.io/apiserver/pkg/authorization/cel"
	genericfeatures "k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/clientcmd"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/yaml"
)

const (
	examplesDir = "../../../../docs/dev/conditional-authorization/examples"
	apolCRD     = "../../../../config/crds/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml"
)

// examplePolicies lists every AuthorizingPolicy example in the docs.
var examplePolicies = []string{"owner-writes.yaml", "label-selector-reads.yaml"}

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
	// A 0s TTL is defaulted to 5m/30s, so caching must be turned off explicitly.
	require.False(t, cfg.Authorizers[2].Webhook.CacheAuthorizedRequests)
	require.False(t, cfg.Authorizers[2].Webhook.CacheUnauthorizedRequests)

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

//nolint:paralleltest // mutates the global feature gate
func TestDocsBootstrapAuthorizationConfiguration(t *testing.T) {
	cfg, err := load.LoadFromFile(filepath.Join(examplesDir, "authorization-config-bootstrap.yaml"))
	require.NoError(t, err)
	known := sets.New("Node", "RBAC", "Webhook", "ABAC", "AlwaysAllow", "AlwaysDeny")
	for _, gate := range []bool{true, false} {
		featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, genericfeatures.ConditionalAuthorization, gate)
		errs := apiservervalidation.ValidateAuthorizationConfiguration(authorizationcel.NewDefaultCompiler(), field.NewPath(""), cfg, known, sets.New("Webhook"))
		require.Empty(t, errs)
	}
	require.Len(t, cfg.Authorizers, 2)
	require.Equal(t, "Node", string(cfg.Authorizers[0].Type))
	require.Equal(t, "RBAC", string(cfg.Authorizers[1].Type))
}

// TestDocsExamplePoliciesMatchCRD validates every example AuthorizingPolicy
// against the generated CRD: OpenAPI schema and x-kubernetes-validations rules,
// as kube-apiserver does on create. Each example must also decode strictly into
// the Go type and compile.
func TestDocsExamplePoliciesMatchCRD(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(apolCRD)
	require.NoError(t, err)
	var v1crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.UnmarshalStrict(raw, &v1crd))
	var crd apiextensions.CustomResourceDefinition
	require.NoError(t, apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(&v1crd, &crd, nil))
	// The internal type keeps a schema shared by all versions in Spec.Validation.
	v := crd.Spec.Validation
	if v == nil {
		require.Len(t, crd.Spec.Versions, 1)
		v = crd.Spec.Versions[0].Schema
	}
	require.NotNil(t, v)
	props := v.OpenAPIV3Schema
	validator, _, err := crdvalidation.NewSchemaValidator(props)
	require.NoError(t, err)
	structural, err := schema.NewStructural(props)
	require.NoError(t, err)
	celValidator := crdcel.NewValidator(structural, true, celconfig.PerCallLimit)
	require.NotNil(t, celValidator)

	for _, name := range examplePolicies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join(examplesDir, name))
			require.NoError(t, err)
			var obj map[string]any
			require.NoError(t, yaml.Unmarshal(raw, &obj))
			require.Equal(t, "policies.kyverno.io/v1alpha1", obj["apiVersion"])
			require.Equal(t, "AuthorizingPolicy", obj["kind"])
			errs := crdvalidation.ValidateCustomResource(nil, obj, validator)
			require.Empty(t, errs, "OpenAPI schema")
			errs, _ = celValidator.Validate(context.Background(), nil, structural, obj, nil, celconfig.RuntimeCELCostBudget)
			require.Empty(t, errs, "x-kubernetes-validations")
			var pol policiesv1alpha1.AuthorizingPolicy
			require.NoError(t, yaml.UnmarshalStrict(raw, &pol))
			_, cerrs := apolcompiler.NewCompiler().Compile(&pol)
			require.Empty(t, cerrs)
		})
	}

	// The schema check must reject an invalid policy, or the test proves nothing.
	bad := map[string]any{
		"apiVersion": "policies.kyverno.io/v1alpha1",
		"kind":       "AuthorizingPolicy",
		"metadata":   map[string]any{"name": "bad"},
		"spec": map[string]any{
			"matchSubjects":  []any{map[string]any{"matchExpressions": []any{map[string]any{"key": "user", "operator": "Exists"}}}},
			"matchResources": map[string]any{},
			"rules":          []any{map[string]any{"name": "r", "effect": "Deny", "expression": "true"}},
		},
	}
	errs, _ := celValidator.Validate(context.Background(), nil, structural, bad, nil, celconfig.RuntimeCELCostBudget)
	require.NotEmpty(t, errs)
}

func TestDocsLabelSelectorReads(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(examplesDir, "label-selector-reads.yaml"))
	require.NoError(t, err)
	var pol policiesv1alpha1.AuthorizingPolicy
	require.NoError(t, yaml.UnmarshalStrict(raw, &pol))
	h := newHandler(t, &pol)
	spec := func(verb string, sel *authorizationv1.LabelSelectorAttributes) *authorizationv1.SubjectAccessReviewSpec {
		return &authorizationv1.SubjectAccessReviewSpec{
			User: "bob", Groups: []string{"team-a"},
			ResourceAttributes: &authorizationv1.ResourceAttributes{Namespace: "a", Verb: verb, Version: "v1", Resource: "configmaps", LabelSelector: sel},
			AuthorizationOptions: &authorizationv1.AuthorizationOptions{
				HandledDecisionTypes: authorizationv1.ConditionalAuthorizationDecisionTypesList(),
			},
		}
	}
	teamA := &authorizationv1.LabelSelectorAttributes{Requirements: []metav1.LabelSelectorRequirement{{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"a"}}}}
	teamB := &authorizationv1.LabelSelectorAttributes{Requirements: []metav1.LabelSelectorRequirement{{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"b"}}}}
	for _, verb := range []string{"list", "watch"} {
		require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeAllow, h.engine.Authorize(context.Background(), spec(verb, teamA), true).Type, verb)
		require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeNoOpinion, h.engine.Authorize(context.Background(), spec(verb, nil), true).Type, verb)
		require.Equal(t, authorizationv1.ConditionsAwareDecisionTypeNoOpinion, h.engine.Authorize(context.Background(), spec(verb, teamB), true).Type, verb)
	}
}

// TestDocsPolicyAuthorRBAC checks that the documented policy-author role grants
// write access to authorizingpolicies only, and binds it to a group.
func TestDocsPolicyAuthorRBAC(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join(examplesDir, "policy-author-rbac.yaml"))
	require.NoError(t, err)
	dec := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(raw)))
	var role rbacv1.ClusterRole
	var binding rbacv1.ClusterRoleBinding
	for {
		doc, err := dec.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if strings.TrimSpace(string(doc)) == "" {
			continue
		}
		switch {
		case bytes.Contains(doc, []byte("kind: ClusterRoleBinding")):
			require.NoError(t, yaml.UnmarshalStrict(doc, &binding))
		case bytes.Contains(doc, []byte("kind: ClusterRole")):
			require.NoError(t, yaml.UnmarshalStrict(doc, &role))
		}
	}
	require.Len(t, role.Rules, 1)
	require.Equal(t, []string{"policies.kyverno.io"}, role.Rules[0].APIGroups)
	require.Equal(t, []string{"authorizingpolicies"}, role.Rules[0].Resources)
	require.Empty(t, role.AggregationRule)
	for k := range role.Labels {
		require.NotContains(t, k, "aggregate-to", "policy-author role must not be aggregated")
	}
	require.Equal(t, role.Name, binding.RoleRef.Name)
	require.Equal(t, "ClusterRole", binding.RoleRef.Kind)
}
