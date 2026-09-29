package factories

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/regcreds"
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

	if creds == nil && len(imagePullSecrets) == 0 {
		return f.globalClient, nil
	}

	// the policy contains extra credentials apart from whats passed in imagePullSecrets
	if creds != nil {
		// turn the array of providers into a slice of credential helper names
		providers := make([]string, len(creds.Providers))
		for i, p := range creds.Providers {
			providers[i] = string(p)
		}

		// create an array of secret names where we will accumulate whats in creds and imagePullSecrets
		// creds.Secrets default to the Kyverno namespace, imagePullSecrets default to the resource namespace,
		// so each list must be prefixed independently before merging.
		secrets := make([]string, 0)
		if f.secretsLister != nil && len(creds.Secrets) > 0 {
			secrets = append(secrets, prefixSecretNamespaces(creds.Secrets, config.KyvernoNamespace())...)
		}
		if len(imagePullSecrets) > 0 {
			secrets = append(secrets, prefixSecretNamespaces(imagePullSecrets, resourceNamespace)...)
		}

		opts := []registryclient.Option{
			registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
			registryclient.WithImagePullSecrets(secrets...),
			registryclient.WithCredentialHelpers(providers...),
			registryclient.WithAllowInsecureRegistry(creds.AllowInsecureRegistry),
			registryclient.WithLogger(logging.GlobalLogger()),
		}

		if creds.TLSClientCert != nil && f.secretsLister != nil {
			certKey := creds.TLSClientCert.CertKey
			if certKey == "" {
				certKey = "tls.crt"
			}
			keyKey := creds.TLSClientCert.KeyKey
			if keyKey == "" {
				keyKey = "tls.key"
			}

			namespace := config.KyvernoNamespace()
			name := creds.TLSClientCert.SecretName
			if parts := strings.SplitN(name, "/", 2); len(parts) == 2 {
				namespace = parts[0]
				name = parts[1]
			}

			secret, err := f.secretsLister.Secrets(namespace).Get(name)
			if err != nil {
				return nil, fmt.Errorf("failed to get mTLS secret %s/%s: %w", namespace, name, err)
			}
			certPEM, ok := secret.Data[certKey]
			if !ok {
				return nil, fmt.Errorf("mTLS secret %s/%s missing %s", namespace, name, certKey)
			}
			keyPEM, ok := secret.Data[keyKey]
			if !ok {
				return nil, fmt.Errorf("mTLS secret %s/%s missing %s", namespace, name, keyKey)
			}

			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("failed to parse mTLS cert for %s/%s: %w", namespace, name, err)
			}

			cloned := regcreds.DefaultTransport.Clone()
			if cloned.TLSClientConfig == nil {
				cloned.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			cloned.TLSClientConfig.Certificates = append(cloned.TLSClientConfig.Certificates, cert)
			opts = append(opts, registryclient.WithTransport(cloned))
		}

		client := registryclient.New(opts...)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	secrets := prefixSecretNamespaces(imagePullSecrets, resourceNamespace)
	client := registryclient.New(
		registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
		registryclient.WithImagePullSecrets(secrets...),
		registryclient.WithLogger(logging.GlobalLogger()),
	)
	return adapters.RegistryClient(client), nil
}

// prefixSecretNamespaces prefixes each secret ref with defaultNamespace unless it already
// uses namespace/name notation.
func prefixSecretNamespaces(secrets []string, defaultNamespace string) []string {
	prefixed := make([]string, len(secrets))
	for i, s := range secrets {
		s = strings.TrimPrefix(s, "/")
		if strings.Contains(s, "/") {
			prefixed[i] = s
		} else {
			prefixed[i] = defaultNamespace + "/" + s
		}
	}
	return prefixed
}
