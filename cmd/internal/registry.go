package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/kyverno/sdk/extensions/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var setRegistryResponseHeaderTimeoutOnce = new(sync.Once)

func setRegistryResponseHeaderTimeout(timeout time.Duration) error {
	if timeout < 0 {
		return fmt.Errorf("registryResponseHeaderTimeout must be non-negative, got %s", timeout)
	}
	setRegistryResponseHeaderTimeoutOnce.Do(func() {
		regcreds.DefaultTransport.ResponseHeaderTimeout = timeout
	})
	return nil
}

func setupRegistryClient(ctx context.Context, logger logr.Logger, client kubernetes.Interface) (registryclient.Client, corev1listers.SecretLister) {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry, "responseHeaderTimeout", registryResponseHeaderTimeout)
	logger.V(2).Info("setup registry client...")
	checkError(logger, setRegistryResponseHeaderTimeout(registryResponseHeaderTimeout), "failed to configure registry transport")
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
		registryclient.WithLogger(logger))

	// WithSecretClient does not change how this lister resolves secrets. It wraps ms so that
	// namespaced ivpol credential resolution can opt into an authorized, bounded GET for a secret
	// in the policy's own namespace, which has no informer. Secrets() is not overridden, so every
	// other consumer - the shared CEL context provider, the ivpol engine's image context and
	// resource imagePullSecrets - keeps ms's cache-only behavior and still gets NotFound for
	// unconfigured namespaces. See pkg/utils/kube/AGENTS.md.
	return registryClient, kubeutils.WithSecretClient(ctx, ms, client.CoreV1())
}

func parseSecretReference(secretRef string, defaultNamespace string) (namespace string, name string) {
	secretRef = strings.TrimPrefix(secretRef, "/")

	parts := strings.SplitN(secretRef, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultNamespace, secretRef
}
