package handlers_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/handlers/mutation"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type recordingCredentialFactory struct {
	calls             int
	credentials       *kyvernov1.ImageRegistryCredentials
	resourceNamespace string
	imagePullSecrets  []string
}

func (f *recordingCredentialFactory) GetClient(_ context.Context, credentials *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string) (engineapi.RegistryClient, error) {
	f.calls++
	f.credentials = credentials
	f.resourceNamespace = resourceNamespace
	f.imagePullSecrets = imagePullSecrets
	return nil, nil
}

// A prior verified verdict lets the allowed cases finish without a registry;
// rejected credentials must fail before either client creation or cache lookup.
type verifiedCredentialImageCache struct {
	imageverifycache.Client
	calls int
}

func (c *verifiedCredentialImageCache) Get(context.Context, metav1.Object, string, string, bool) (bool, error) {
	c.calls++
	return true, nil
}

func TestVerifyImageHandlers_ConfineCredentialsBeforeCachedVerification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		cluster       bool
		nilPolicy     bool
		noCredentials bool
		secret        string
		variable      string
		expected      string
		errorContains string
	}{
		{name: "bare secret", namespace: "tenant-a", secret: "pull-secret", expected: "tenant-a/pull-secret"},
		{name: "same namespace", namespace: "tenant-a", secret: "tenant-a/pull-secret", expected: "tenant-a/pull-secret"},
		{name: "foreign namespace", namespace: "tenant-a", secret: "tenant-b/pull-secret", errorContains: "instead of policy namespace"},
		{name: "installation namespace", namespace: "tenant-a", secret: "kyverno/pull-secret", errorContains: "instead of policy namespace"},
		{name: "malformed secret", namespace: "tenant-a", secret: "tenant-a/pull/extra", errorContains: "invalid name"},
		{name: "empty secret name", namespace: "tenant-a", secret: "tenant-a/", errorContains: "empty name"},
		{name: "malformed policy namespace", namespace: "tenant/a", secret: "pull-secret", errorContains: "policy namespace \"tenant/a\" is invalid"},
		{name: "missing policy namespace", secret: "pull-secret", errorContains: "policy namespace must not be empty"},
		{name: "missing namespace without credentials", noCredentials: true, errorContains: "policy namespace must not be empty"},
		{name: "missing policy", nilPolicy: true, secret: "pull-secret", errorContains: "policy scope must not be nil"},
		{name: "bare secret variable", namespace: "tenant-a", secret: "{{ credentialSecret }}", variable: "pull-secret", expected: "tenant-a/pull-secret"},
		{name: "foreign namespace after substitution", namespace: "tenant-a", secret: "{{ credentialSecret }}", variable: "tenant-b/pull-secret", errorContains: "instead of policy namespace"},
		{name: "cluster policy bare secret", cluster: true, secret: "pull-secret", expected: "pull-secret"},
		{name: "cluster policy foreign namespace", cluster: true, secret: "tenant-b/pull-secret", expected: "tenant-b/pull-secret"},
		{name: "no explicit credentials", namespace: "tenant-a", noCredentials: true},
	}

	for _, handlerKind := range []string{"mutation"} {
		t.Run(handlerKind, func(t *testing.T) {
			t.Parallel()
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					var credentials *kyvernov1.ImageRegistryCredentials
					if !test.noCredentials {
						credentials = &kyvernov1.ImageRegistryCredentials{
							Secrets:               []string{test.secret},
							AllowInsecureRegistry: true,
						}
					}
					rule := kyvernov1.Rule{
						Name: "verify-image",
						VerifyImages: []kyvernov1.ImageVerification{{
							ImageReferences: []string{"ghcr.io/verified/*"},
							Required:        true,
							VerifyDigest:    true,
							UseCache:        true,
							Attestors: []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{{
								Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: "cached-key"},
							}}}},
							ImageRegistryCredentials: credentials,
						}},
					}
					var policy kyvernov1.PolicyInterface
					if test.cluster {
						policy = &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "verify-image"}}
					} else if !test.nilPolicy {
						policy = &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Name: "verify-image", Namespace: test.namespace}}
					}
					resource := unstructured.Unstructured{Object: map[string]interface{}{
						"apiVersion": "v1", "kind": "Pod",
						"metadata": map[string]interface{}{"name": "app", "namespace": "resource-ns"},
						"spec": map[string]interface{}{
							"containers": []interface{}{map[string]interface{}{
								"name": "app", "image": "ghcr.io/verified/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
							}},
							"imagePullSecrets": []interface{}{map[string]interface{}{"name": "resource-pull-secret"}},
						},
					}}
					cfg := config.NewDefaultConfiguration(false)
					pc, err := policycontext.NewPolicyContext(jmespath.New(cfg), resource, kyvernov1.Create, nil, cfg)
					require.NoError(t, err)
					pc = pc.WithPolicy(policy)
					if test.variable != "" {
						require.NoError(t, pc.JSONContext().AddVariable("credentialSecret", test.variable))
					}
					factory := &recordingCredentialFactory{}
					cache := &verifiedCredentialImageCache{}
					var handler handlers.Handler
					handler, err = mutation.NewMutateImageHandler(pc, resource, rule, cfg, factory, cache, &engineapi.ImageVerificationMetadata{}, nil, true)
					require.NoError(t, err)
					require.NotNil(t, handler)
					_, responses := handler.Process(context.Background(), logr.Discard(), pc, resource, rule, nil, nil)
					require.Len(t, responses, 1)
					if test.errorContains != "" {
						assert.Equal(t, engineapi.RuleStatusError, responses[0].Status())
						assert.Contains(t, responses[0].Message(), test.errorContains)
						assert.Zero(t, factory.calls, "invalid scope must be rejected before loading registry credentials")
						assert.Zero(t, cache.calls, "a cached verdict must not bypass credential scope validation")
					} else {
						assert.Equal(t, engineapi.RuleStatusPass, responses[0].Status(), responses[0].Message())
						assert.Equal(t, 1, factory.calls)
						assert.Equal(t, 1, cache.calls)
						assert.Equal(t, "resource-ns", factory.resourceNamespace)
						assert.Equal(t, []string{"resource-pull-secret"}, factory.imagePullSecrets)
						if test.noCredentials {
							assert.Nil(t, factory.credentials)
						} else {
							require.NotNil(t, factory.credentials)
							assert.Equal(t, []string{test.expected}, factory.credentials.Secrets)
							assert.True(t, factory.credentials.AllowInsecureRegistry)
						}
					}
					if credentials != nil {
						assert.Equal(t, []string{test.secret}, credentials.Secrets, "the policy's credential references must not be mutated")
					}
				})
			}
		})
	}
}
