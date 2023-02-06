package internal

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func LoadContext(ctx context.Context, factory engineapi.ContextLoaderFactory, contextEntries []kyvernov1.ContextEntry, pContext engineapi.PolicyContext, ruleName string) error {
	return factory(pContext, ruleName).Load(ctx, contextEntries, pContext.JSONContext())
}
