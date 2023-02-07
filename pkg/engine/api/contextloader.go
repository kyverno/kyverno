package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

// ContextLoaderFactory provides a ContextLoader given a policy context and rule name
type ContextLoaderFactory = func(pContext PolicyContext, ruleName string) ContextLoader

// ContextLoader abstracts the mechanics to load context entries in the underlying json context
type ContextLoader interface {
	Load(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error
}
