package internal

import (
	"context"
	"flag"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/registryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// setupRegistryClient hands the same lister to namespaced ivpol credential resolution and to every
// other secret consumer (the legacy image-verification engine, the shared CEL context provider,
// resource imagePullSecrets). Wrapping multiLister with WithSecretClient must therefore not widen
// secret access on its own: only an explicit ScopeSecretLister may reach the live API, and only
// inside the policy's own namespace. See pkg/utils/kube/AGENTS.md.
func TestRegistrySecretListerKeepsCacheOnlyDefault(t *testing.T) {
	t.Parallel()
	const installNamespace, policyNamespace = "kyverno", "tenant-a"

	newLister := func(t *testing.T) (corev1listers.SecretLister, *kubefake.Clientset) {
		t.Helper()
		indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		cached := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "regcred", Namespace: installNamespace},
			Data:       map[string][]byte{"source": []byte("cache")},
		}
		require.NoError(t, indexer.Add(cached))
		live := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "regcred", Namespace: policyNamespace},
			Data:       map[string][]byte{"source": []byte("live")},
		}
		client := kubefake.NewSimpleClientset(live)
		ms := &multiLister{listersMap: map[string]corev1listers.SecretLister{
			installNamespace: corev1listers.NewSecretLister(indexer),
		}}
		return kubeutils.WithSecretClient(context.Background(), ms, client.CoreV1()), client
	}

	t.Run("configured namespace still resolves from the informer", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		secret, err := lister.Secrets(installNamespace).Get("regcred")
		require.NoError(t, err)
		assert.Equal(t, "cache", string(secret.Data["source"]))
		assert.Empty(t, client.Actions(), "a cache hit must not reach the API server")
	})

	t.Run("unconfigured namespace stays NotFound without an API call", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := lister.Secrets(policyNamespace).Get("regcred")
		require.True(t, apierrors.IsNotFound(err), "unscoped lookups must keep multiLister behavior, got %v", err)
		assert.Empty(t, client.Actions(), "unscoped consumers must never trigger a tenant secret read")
	})

	t.Run("scoped lookup reaches the policy namespace", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		secret, err := kubeutils.ScopeSecretLister(lister, policyNamespace).Secrets(policyNamespace).Get("regcred")
		require.NoError(t, err)
		assert.Equal(t, "live", string(secret.Data["source"]))
		require.Len(t, client.Actions(), 1)
		assert.Equal(t, "get", client.Actions()[0].GetVerb())
		assert.Equal(t, policyNamespace, client.Actions()[0].GetNamespace())
	})

	t.Run("scoped lookup rejects the installation namespace", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := kubeutils.ScopeSecretLister(lister, policyNamespace).Secrets(installNamespace).Get("regcred")
		require.True(t, apierrors.IsForbidden(err), "a namespaced policy must not borrow installation credentials, got %v", err)
		assert.Empty(t, client.Actions())
	})

	t.Run("cluster scoped policies are left unwrapped", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := kubeutils.ScopeSecretLister(lister, "").Secrets(policyNamespace).Get("regcred")
		require.True(t, apierrors.IsNotFound(err), "an empty namespace must not enable live reads, got %v", err)
		assert.Empty(t, client.Actions())
	})
}

// registryEgressOptions is the only thing wiring the guard into the process-wide registry
// client, and every image fetch in the controllers inherits it from there. Exercising the
// options beats counting them: a non-empty slice says nothing about whether the guard reached
// them. A literal metadata address needs no DNS and no network, so the transport rejects it
// before the first byte is written.
func TestRegistryEgressOptionsCarryTheGuard(t *testing.T) {
	previous := egress.Global()
	t.Cleanup(func() { egress.RestoreGlobal(previous) })
	require.NoError(t, egress.SetGlobal(egress.Config{Mode: egress.ModeEnforce}))

	blocked, err := name.ParseReference("169.254.169.254/app:v1")
	require.NoError(t, err)
	opts, _ := registryclient.NewRemoteOptions(registryEgressOptions()...)
	_, err = gcrremote.Get(blocked, opts...)
	require.ErrorIs(t, err, egress.ErrAddressBlocked, "the registry client options did not carry the guard")
}

func TestEgressConfigValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		allowlist string
		mode      string
		wantError bool
	}{
		{name: "defaults"},
		{name: "audit", mode: "audit"},
		{name: "enforce", mode: "enforce", allowlist: "registry.internal,10.0.0.0/8,fd00::/8"},
		{name: "invalid mode", mode: "disabled", wantError: true},
		{name: "wildcard", allowlist: "*.internal", wantError: true},
		{name: "URL", allowlist: "https://registry.internal", wantError: true},
		{name: "invalid CIDR", allowlist: "10.0.0.0/33", wantError: true},
		{name: "empty list element", allowlist: "registry.internal,,10.0.0.1", wantError: true},
		{name: "trailing comma", allowlist: "registry.internal,", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := egress.ValidateConfig(egress.Config{
				Allowlist: egress.ParseAllowlist(tc.allowlist),
				Mode:      tc.mode,
			})
			require.Equal(t, tc.wantError, err != nil, "error = %v", err)
		})
	}
}

// Egress flags must reach controllers that only read image metadata, while the credential
// flags stay behind WithRegistryClient so the cleanup controller does not get Secret
// informers or the RBAC they need.
func TestRegistryFlagRegistration(t *testing.T) {
	originalFlags := flag.CommandLine
	originalAllowlist, originalMode := privateRegistryAllowlist, privateRegistryEgressMode
	originalRanges := privateRegistryRanges
	originalSecrets, originalHelpers := imagePullSecrets, registryCredentialHelpers
	originalInsecure, originalPayloadLimit := allowInsecureRegistry, imageVerifyPayloadLimit
	t.Cleanup(func() {
		flag.CommandLine = originalFlags
		privateRegistryAllowlist, privateRegistryEgressMode = originalAllowlist, originalMode
		privateRegistryRanges = originalRanges
		imagePullSecrets, registryCredentialHelpers = originalSecrets, originalHelpers
		allowInsecureRegistry, imageVerifyPayloadLimit = originalInsecure, originalPayloadLimit
	})
	for _, test := range []struct {
		name       string
		options    []ConfigurationOption
		wantEgress bool
		wantClient bool
	}{
		{name: "disabled"},
		{name: "egress only", options: []ConfigurationOption{WithRegistryEgress()}, wantEgress: true},
		{name: "registry client implies egress", options: []ConfigurationOption{WithRegistryClient()}, wantEgress: true, wantClient: true},
		{name: "both options", options: []ConfigurationOption{WithRegistryEgress(), WithRegistryClient()}, wantEgress: true, wantClient: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(test.name, flag.ContinueOnError)
			configuration := NewConfiguration(test.options...)
			require.Equal(t, test.wantEgress, configuration.UsesRegistryEgress())
			require.Equal(t, test.wantClient, configuration.UsesRegistryClient())
			initRegistryFlags(configuration)
			for _, flagName := range []string{"privateRegistryAllowlist", "privateRegistryEgressMode", "privateRegistryRanges"} {
				require.Equal(t, test.wantEgress, flag.Lookup(flagName) != nil, flagName)
			}
			for _, flagName := range []string{"imagePullSecrets", "registryCredentialHelpers", "allowInsecureRegistry", "imageVerifyPayloadLimit"} {
				require.Equal(t, test.wantClient, flag.Lookup(flagName) != nil, flagName)
			}
			if test.wantEgress {
				require.Equal(t, egress.ModeAudit, privateRegistryEgressMode)
				require.Empty(t, privateRegistryAllowlist)
				require.NoError(t, flag.CommandLine.Parse([]string{"--privateRegistryEgressMode=enforce", "--privateRegistryAllowlist=10.20.30.0/24"}))
				require.Equal(t, "enforce", privateRegistryEgressMode)
				require.Equal(t, "10.20.30.0/24", privateRegistryAllowlist)
				// The range flag defaults to the shipped list rather than empty, so a Kyverno
				// installed without the chart keeps the protection the chart would render. An
				// operator clearing it passes the flag explicitly, and the parsed result stays
				// non-nil so New reads it as "cleared" and not as "unset".
				require.Equal(t, egress.DefaultPrivateRanges, egress.ParseRanges(privateRegistryRanges))
				require.NoError(t, flag.CommandLine.Parse([]string{"--privateRegistryRanges="}))
				require.Equal(t, []string{}, egress.ParseRanges(privateRegistryRanges))
			}
		})
	}
}

func TestParseAllowlist(t *testing.T) {
	t.Parallel()
	require.Nil(t, egress.ParseAllowlist(""))
	require.Nil(t, egress.ParseAllowlist("   "))
	require.Equal(t, []string{"a", "b"}, egress.ParseAllowlist("a,b"))
	// A single empty element is preserved so New rejects it rather than silently ignoring it.
	require.Equal(t, []string{"a", "", "b"}, egress.ParseAllowlist("a,,b"))
}
