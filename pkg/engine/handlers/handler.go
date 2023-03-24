package handlers

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type Handler interface {
	Process(
		context.Context,
		logr.Logger,
		engineapi.PolicyContext,
		kyvernov1.Rule,
	) *engineapi.RuleResponse
}
