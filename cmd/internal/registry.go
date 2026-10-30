package internal

import (
	"context"
	"errors"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/kyverno/sdk/extensions/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// egressConfig reads the operator flags. check() has already rejected invalid values, so
// callers can treat SetGlobal failures here as a startup error.
func egressConfig() egress.Config {
	return egress.Config{
		Allowlist: egress.ParseAllowlist(privateRegistryAllowlist),
		Mode:      privateRegistryEgressMode,
	}
}

// registryEgressOptions guards every request the registry client makes: the transport
// validates each destination, including token challenges and redirects, and the keychain
// wrapper validates before credentials are resolved for a registry.
func registryEgressOptions() []registryclient.Option {
	guard := egress.Global()
	return []registryclient.Option{
		registryclient.WithTransport(guard.Transport()),
		registryclient.WithKeychainWrapper(guard.GuardKeychain),
	}
}

// setupRegistryEgress installs the guard for controllers that read image metadata without
// needing registry credentials, so they get no credential helpers, image-pull Secret
// informers, or the RBAC those require.
func setupRegistryEgress(logger logr.Logger) {
	checkError(logger, egress.SetGlobal(egressConfig()), "failed to configure registry egress")
	registryclient.SetupGlobalRegistryClient(nil, "", "", "", false,
		append(registryEgressOptions(), registryclient.WithLogger(logger))...)
}

func setupRegistryClient(ctx context.Context, logger logr.Logger, client kubernetes.Interface) (registryclient.Client, corev1listers.SecretLister) {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.V(2).Info("setup registry client...")
	checkError(logger, egress.SetGlobal(egressConfig()), "failed to configure registry egress")
	ms := &multiLister{
		listersMap: make(map[string]corev1listers.SecretLister),
	}
	addListerForNamespace := func(ns string) {
		factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(ns))
		secretLister := factory.Core().V1().Secrets().Lister()
		if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
			checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		}
		ms.listersMap[ns] = secretLister
	}
	addListerForNamespace(config.KyvernoNamespace())

	for s := range strings.SplitSeq(imagePullSecrets, ",") {
		namespace, _ := parseSecretReference(s, config.KyvernoNamespace())
		if _, exists := ms.listersMap[namespace]; !exists {
			addListerForNamespace(namespace)
		}
	}

	registryClient := registryclient.SetupGlobalRegistryClient(ms, config.KyvernoNamespace(),
		imagePullSecrets,
		registryCredentialHelpers, allowInsecureRegistry,
		append(registryEgressOptions(), registryclient.WithLogger(logger))...)

	return registryClient, ms
}

func parseSecretReference(secretRef string, defaultNamespace string) (namespace string, name string) {
	secretRef = strings.TrimPrefix(secretRef, "/")

	parts := strings.SplitN(secretRef, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultNamespace, secretRef
}
