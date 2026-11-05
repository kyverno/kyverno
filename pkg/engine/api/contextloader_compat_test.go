package api_test

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

// External consumers can provide and receive factories with the release-1.19
// PolicyScope signature without adapters or function conversions.
var _ engineapi.ContextLoaderFactory = func(engineapi.PolicyScope, kyvernov1.Rule) engineapi.ContextLoader {
	return nil
}

var _ func(engineapi.PolicyScope, kyvernov1.Rule) engineapi.ContextLoader = engineapi.ContextLoaderFactory(nil)
