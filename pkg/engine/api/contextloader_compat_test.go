package api_test

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

// External consumers can provide and receive factories with the original
// PolicyInterface signature without adapters or function conversions.
var _ engineapi.ContextLoaderFactory = func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.ContextLoader {
	return nil
}

var _ func(kyvernov1.PolicyInterface, kyvernov1.Rule) engineapi.ContextLoader = engineapi.ContextLoaderFactory(nil)
