package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type RegistryClientFactory interface {
	GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string) (RegistryClient, error)
}

type Initializer = func(jsonContext Interface) error
