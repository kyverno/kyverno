package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

type ContextLoaderFactory = func(pContext PolicyContext, ruleName string) ContextLoader

type ContextLoader interface {
	Load(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error
}
