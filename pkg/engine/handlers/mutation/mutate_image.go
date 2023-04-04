package mutation

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mutateImageHandler struct {
	configuration config.Configuration
	rclient       registryclient.Client
	ivm           *engineapi.ImageVerificationMetadata
	images        []apiutils.ImageInfo
}

func NewMutateImageHandler(
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	configuration config.Configuration,
	rclient registryclient.Client,
	ivm *engineapi.ImageVerificationMetadata,
) (handlers.Handler, error) {
	if len(rule.VerifyImages) == 0 {
		return nil, nil
	}
	ruleImages, _, err := engineutils.ExtractMatchingImages(resource, policyContext.JSONContext(), rule, configuration)
	if err != nil {
		return nil, err
	}
	if len(ruleImages) == 0 {
		return nil, nil
	}
	return mutateImageHandler{
		configuration: configuration,
		rclient:       rclient,
		ivm:           ivm,
		images:        ruleImages,
	}, nil
}

func (h mutateImageHandler) Process(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	resource unstructured.Unstructured,
	rule kyvernov1.Rule,
	contextLoader engineapi.EngineContextLoader,
) (unstructured.Unstructured, []engineapi.RuleResponse) {
	jsonContext := policyContext.JSONContext()
	ruleCopy, err := substituteVariables(rule, jsonContext, logger)
	if err != nil {
		return resource, handlers.WithResponses(
			engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to substitute variables", err),
		)
	}
	iv := internal.NewImageVerifier(logger, h.rclient, policyContext, *ruleCopy, h.ivm)
	var engineResponses []*engineapi.RuleResponse
	for _, imageVerify := range ruleCopy.VerifyImages {
		engineResponses = append(engineResponses, iv.Verify(ctx, imageVerify, h.images, h.configuration)...)
	}
	return resource, handlers.WithResponses(engineResponses...)
}

func substituteVariables(rule kyvernov1.Rule, ctx enginecontext.EvalInterface, logger logr.Logger) (*kyvernov1.Rule, error) {
	// remove attestations as variables are not substituted in them
	ruleCopy := *rule.DeepCopy()
	for i := range ruleCopy.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = nil
	}
	var err error
	ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, ruleCopy)
	if err != nil {
		return nil, err
	}
	// replace attestations
	for i := range rule.VerifyImages {
		ruleCopy.VerifyImages[i].Attestations = rule.VerifyImages[i].Attestations
	}
	return &ruleCopy, nil
}
