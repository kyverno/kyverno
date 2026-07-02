package internal

import (
	"context"
	"errors"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func setupRegistryClient(ctx context.Context, logger logr.Logger, client kubernetes.Interface) (registryclient.Client, corev1listers.SecretLister) {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.V(2).Info("setup registry client...")
	ms := &multiLister{
		listersMap: make(map[string]corev1listers.SecretLister),
	}

	for s := range strings.SplitSeq(imagePullSecrets, ",") {
		namespace, _ := parseSecretReference(s, config.KyvernoNamespace())
		if _, exists := ms.listersMap[namespace]; !exists {
			factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(namespace))
			secretLister := factory.Core().V1().Secrets().Lister()
			if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
				checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			}
			ms.listersMap[namespace] = secretLister
		}
	}

	registryClient := registryclient.SetupGlobalRegistryClient(ms, config.KyvernoNamespace(),
		imagePullSecrets,
		registryCredentialHelpers, allowInsecureRegistry)

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
