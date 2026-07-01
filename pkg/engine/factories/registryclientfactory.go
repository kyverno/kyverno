package factories

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func DefaultRegistryClientFactory(globalClient engineapi.RegistryClient, secretsLister corev1listers.SecretLister) engineapi.RegistryClientFactory {
	return &registryClientFactory{
		globalClient:  globalClient,
		secretsLister: secretsLister,
	}
}

type registryClientFactory struct {
	globalClient  engineapi.RegistryClient
	secretsLister corev1listers.SecretLister
}

func (f *registryClientFactory) GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string) (engineapi.RegistryClient, error) {
	if resourceNamespace == "" {
		resourceNamespace = config.KyvernoNamespace()
	}

	if len(imagePullSecrets) == 0 {
		client := registryclient.New(f.secretsLister, config.KyvernoNamespace(), "", "", creds.AllowInsecureRegistry)
		return adapters.RegistryClient(client), nil
	}

	if creds != nil {
		providers := ""
		if len(creds.Providers) > 0 {
			for i, helper := range creds.Providers {
				providers += string(helper)
				// only add a coma if we still didn't reach the end
				if i <= len(creds.Providers)-1 {
					providers += ","
				}
			}
		}

		secrets := make([]string, 0)
		if creds != nil && f.secretsLister != nil && len(creds.Secrets) > 0 {
			secrets = append(secrets, creds.Secrets...)
		}

		secretsJoined := ""
		if f.secretsLister != nil && len(secrets) > 0 {
			// Support namespace/name notation with Kyverno namespace as default
			secretsJoined = strings.Join(secrets, ",")
		}
		client := registryclient.New(f.secretsLister, resourceNamespace, secretsJoined, "", creds.AllowInsecureRegistry)
		return adapters.RegistryClient(client), nil
	}
	return f.globalClient, nil
}
