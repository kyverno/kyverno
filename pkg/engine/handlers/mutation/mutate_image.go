package mutation

import (
	"context"

	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/handlers"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/registryclient"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/mattbaird/jsonpatch"
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
	var patches []jsonpatch.JsonPatchOperation
	for _, response := range engineResponses {
		patches = append(patches, response.Patches()...)
	}
	if len(patches) != 0 {
		patch := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
		decoded, err := json_patch.DecodePatch(patch)
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to decode patch", err),
			)
		}
		options := &json_patch.ApplyOptions{SupportNegativeIndices: true, AllowMissingPathOnRemove: true, EnsurePathExistsOnAdd: true}
		resourceBytes, err := resource.MarshalJSON()
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to marshal resource", err),
			)
		}
		patchedResourceBytes, err := decoded.ApplyWithOptions(resourceBytes, options)
		if err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to apply patch", err),
			)
		}
		if err := resource.UnmarshalJSON(patchedResourceBytes); err != nil {
			return resource, handlers.WithResponses(
				engineapi.RuleError(rule.Name, engineapi.ImageVerify, "failed to unmarshal resource", err),
			)
		}
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
