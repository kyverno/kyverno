package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

type RegistryClientFactory interface {
	GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials) (RegistryClient, error)
}

type Initializer func(jsonContext enginecontext.Interface) error

// ContextLoaderFactory provides a ContextLoader given a policy and rule name
type ContextLoaderFactory = func(policyName, ruleName string) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(
		ctx context.Context,
		contextEntries []kyvernov1.ContextEntry,
		jsonContext enginecontext.Interface,
	) error
}
