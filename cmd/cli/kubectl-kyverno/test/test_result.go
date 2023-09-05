package test

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type TestResult struct {
	EngineResponses []engineapi.EngineResponse
	Results         []api.TestResults
	Err             error
}
