package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type Initializer = func(jsonContext enginecontext.Interface) error

// ContextLoaderFactory provides a ContextLoader given a policy context and rule name
type ContextLoaderFactory = func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(
		ctx context.Context,
		jp jmespath.Interface,
		client dclient.Interface,
		rclient registryclient.Client,
		contextEntries []kyvernov1.ContextEntry,
		jsonContext enginecontext.Interface,
	) error
}
