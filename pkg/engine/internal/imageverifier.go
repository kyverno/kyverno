package internal

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/image/verifiers/cpol/cosign"
	"github.com/kyverno/kyverno/pkg/image/verifiers/cpol/notary"
	apiutils "github.com/kyverno/kyverno/pkg/utils/api"
	stringutils "github.com/kyverno/kyverno/pkg/utils/strings"
	"github.com/kyverno/kyverno/pkg/validation/policy"
	"go.uber.org/multierr"
	"gomodules.xyz/jsonpatch/v2"
)

type ImageVerifier interface {
	Verify(context.Context, kyvernov1.ImageVerification, []apiutils.ImageInfo, config.Configuration) ([]jsonpatch.JsonPatchOperation, []*engineapi.RuleResponse)
}

type imageVerifier struct {
	logger        logr.Logger
	rclient       engineapi.RegistryClient
	ivCache       imageverifycache.Client
	policyContext engineapi.PolicyContext
	rule          kyvernov1.Rule
	origRule      kyvernov1.Rule
	ivm           *engineapi.ImageVerificationMetadata
}

func NewImageVerifier(
	logger logr.Logger,
	rclient engineapi.RegistryClient,
	ivCache imageverifycache.Client,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
	ivm *engineapi.ImageVerificationMetadata,
	origRule ...kyvernov1.Rule,
) ImageVerifier {
	var r kyvernov1.Rule
	if len(origRule) > 0 {
		r = origRule[0]
	} else {
		r = rule
	}
	return &imageVerifier{
		logger:        logger,
		rclient:       rclient,
		ivCache:       ivCache,
		policyContext: policyContext,
		rule:          rule,
		origRule:      r,
		ivm:           ivm,
	}
}

// verify applies policy rules to each matching image. The policy rule results and annotation patches are
// added to tme imageVerifier `resp` and `ivm` fields.
func (iv *imageVerifier) Verify(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	matchedImageInfos []apiutils.ImageInfo,
	cfg config.Configuration,
) ([]jsonpatch.JsonPatchOperation, []*engineapi.RuleResponse) {
	var responses []*engineapi.RuleResponse
	var patches []jsonpatch.JsonPatchOperation

	// for backward compatibility
	imageVerify = *imageVerify.Convert()

	for _, imageInfo := range matchedImageInfos {
		p, r := iv.verifyMatchedImage(ctx, imageVerify, imageInfo, cfg)
		patches = append(patches, p...)
		responses = append(responses, r...)
	}
	return patches, responses
}

func (iv *imageVerifier) verifyMatchedImage(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	cfg config.Configuration,
) ([]jsonpatch.JsonPatchOperation, []*engineapi.RuleResponse) {
	var responses []*engineapi.RuleResponse
	var patches []jsonpatch.JsonPatchOperation

	image := imageInfo.String()

	declaredContextNames := iv.getDeclaredContextNames()
	cacheScope, err := VerifyImagesCacheScope(iv.rule.Name, imageVerify, iv.policyContext.JSONContext(), declaredContextNames)
	useCache := imageVerify.UseCache && err == nil
	if err != nil {
		iv.logger.Error(err, "failed to derive cache scope, caching disabled for this rule entry", "ruleName", iv.rule.Name)
	}
	if useCache && (hasUnbindableKeyMaterial(&imageVerify) || hasUnresolvedConditionVariables(&imageVerify, iv.policyContext.JSONContext(), declaredContextNames)) {
		iv.logger.V(2).Info("caching disabled: entry verifies with key material or conditions the cache key cannot bind", "ruleName", iv.rule.Name)
		useCache = false
	}

	imageUseCache := useCache &&
		(len(imageVerify.Attestors) > 0 || len(imageVerify.Attestations) > 0) &&
		matchReferences(imageVerify.ImageReferences, image) &&
		!matchReferences(imageVerify.SkipImageReferences, image)
	if useCache && !imageUseCache {
		iv.logger.V(2).Info("caching disabled for this entry/image", "image", image, "ruleName", iv.rule.Name)
	}

	cacheUsable := iv.ivCache != nil && iv.ivCache.IsEnabled() && imageUseCache
	cacheInfo := imageInfo
	if cacheInfo.Digest == "" && cacheUsable && iv.rclient != nil {
		if desc, err := iv.rclient.FetchImageDescriptor(ctx, image); err == nil && desc != nil {
			cacheInfo.Digest = desc.Digest.String()
		} else if err != nil {
			iv.logger.V(4).Info("failed to resolve image digest for cache lookup", "image", image, "error", err.Error())
		}
	}
	cacheKey := cacheInfo.String()

	isInCache := false
	if iv.ivCache != nil && imageUseCache {
		found, err := iv.ivCache.Get(ctx, iv.policyContext.Policy(), cacheScope, cacheKey, imageUseCache)
		if err != nil {
			iv.logger.Error(err, "error occurred during cache get", "image", cacheKey)
		} else {
			isInCache = found
		}
	}

	var ruleResp *engineapi.RuleResponse
	var digest string
	if isInCache {
		iv.logger.V(2).Info("cache entry found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", cacheKey)
		digest = cacheInfo.Digest
		if iv.rule.HasValidateImageVerification() {
			if err := iv.evaluateRuleValidation(ctx); err != nil {
				msg := fmt.Sprintf("validation in verifyImages failed: %v", err)
				ruleResp = engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties)
			} else {
				ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "verified from cache", iv.rule.ReportProperties)
			}
		} else {
			ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "verified from cache", iv.rule.ReportProperties)
		}
	} else {
		iv.logger.V(2).Info("cache entry not found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
		ruleResp, digest = iv.verifyImage(ctx, imageVerify, imageInfo, cfg)
		if ruleResp != nil && ruleResp.Status() == engineapi.RuleStatusPass {
			if iv.ivCache != nil && imageUseCache {
				setInfo := imageInfo
				if digest != "" {
					setInfo.Digest = digest
				}
				setKey := setInfo.String()
				setted, err := iv.ivCache.Set(ctx, iv.policyContext.Policy(), cacheScope, setKey, imageUseCache)
				if err != nil {
					iv.logger.Error(err, "error occurred during cache set", "image", setKey)
				} else {
					if setted {
						iv.logger.V(4).Info("successfully set cache", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", setKey)
					}
				}
			}
		}
	}

	if imageVerify.MutateDigest {
		patch, retrievedDigest, err := iv.handleMutateDigest(ctx, digest, imageInfo)
		if err != nil {
			responses = append(responses, engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, "failed to update digest", err, iv.rule.ReportProperties))
		} else if patch != nil {
			if ruleResp == nil {
				ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "mutated image digest", iv.rule.ReportProperties)
			}
			patches = append(patches, *patch)
			imageInfo.Digest = retrievedDigest
			image = imageInfo.String()
		}
	}

	if ruleResp != nil {
		if len(imageVerify.Attestors) > 0 || len(imageVerify.Attestations) > 0 {
			iv.addImageVerificationMetadata(image, ruleStatusToImageVerificationStatus(ruleResp.Status()))
			if ruleResp.Status() == engineapi.RuleStatusPass && iv.rule.HasValidateImageVerification() {
				if err := iv.evaluateRuleValidation(ctx); err != nil {
					msg := fmt.Sprintf("validation in verifyImages failed: %v", err)
					ruleResp = engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties)
				}
			}
		}
		responses = append(responses, ruleResp)
	}
	return patches, responses
}

// hasUnbindableKeyMaterial reports whether any attestor verifies with key
// material the cache key cannot bind. keys.secret, keys.kms, and a publicKeys
// value that is a reference rather than an inline PEM all name material that is
// fetched during verification, after the key is derived, so rotating what sits
// behind the name leaves the hash unchanged and the old verdict live.
//
// The key binds the reference, which is the right thing for everything the
// policy author wrote, but it cannot bind bytes it never sees. Rather than
// serve a verdict whose inputs are only partly keyed, skip the cache for these
// entries: a value that cannot be bound should not be cached.


// hasResourceDependentGate reports whether a cached pass for this entry could
// skip a check that is evaluated against the admission request rather than
// against the image alone. Such a check reads the request from the JSON
// context, so a verdict cached for one resource says nothing about the next.
//
// The two gates have different scopes. Attestation conditions belong to the
// entry declaring them. validate.deny does not: verifyImage runs it in a loop
// over every entry of the rule, so the entry that executes a deny need not be
// the one that declares it, and an entry with no attestations never reaches
// that loop at all. Any deny anywhere in the rule therefore makes the whole
// rule uncacheable.
func hasResourceDependentGate(rule kyvernov1.Rule, imageVerify kyvernov1.ImageVerification) bool {
	for _, entry := range rule.VerifyImages {
		if entry.Validation.Deny != nil {
			return true
		}
	}
	if imageVerify.Validation.Deny != nil {
		return true
	}
	for _, attestation := range imageVerify.Attestations {
		if len(attestation.Conditions) > 0 {
			return true
		}
	}
	return false
}

// VerifyImagesCacheScope binds a cache entry to the verification requirements
// of one verifyImages entry, not merely to the rule containing it.
//
// Attestor material reaches this point already variable-substituted, so
// rotating a key supplied through a {{ }} reference changes the scope and
// invalidates the prior verdict. Material named by reference does not: keys.secret
// and keys.kms are dereferenced during verification, after this hash is taken,
// so rotating the Secret or the KMS key leaves the scope unchanged and the old
// verdict usable until the entry expires.
func VerifyImagesCacheScope(ruleName string, imageVerify kyvernov1.ImageVerification, opts ...any) (string, error) {
	return attestorCacheRule(ruleName, &imageVerify, opts...)
}

func attestorCacheRule(ruleName string, imageVerify *kyvernov1.ImageVerification, opts ...any) (string, error) {
	if imageVerify == nil {
		return "", errors.New("imageVerify cannot be nil")
	}
	scoped := *imageVerify.DeepCopy()
	scoped.MutateDigest = false
	scoped.UseCache = false

	for i := range scoped.Attestations {
		if scoped.Attestations[i].Type == "" && scoped.Attestations[i].PredicateType != "" {
			scoped.Attestations[i].Type = scoped.Attestations[i].PredicateType
		}
	}
	var evalCtx enginecontext.EvalInterface
	var declaredNames map[string]bool
	for _, opt := range opts {
		switch v := opt.(type) {
		case enginecontext.EvalInterface:
			evalCtx = v
		case map[string]bool:
			declaredNames = v
		}
	}
	if evalCtx != nil {
		bindResolvedConditions(&scoped, evalCtx, declaredNames)
	}
	encoded, err := json.Marshal(scoped)
	if err != nil {
		return "", fmt.Errorf("failed to encode image verification rule %q: %w", ruleName, err)
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%s;%s", ruleName, hex.EncodeToString(sum[:])), nil
}

func (iv *imageVerifier) addImageVerificationMetadata(image string, status engineapi.ImageVerificationMetadataStatus) {
	policy := iv.policyContext.Policy()
	iv.ivm.Add(image, status)
	iv.ivm.AddScoped(policy.GetNamespace(), policy.GetName(), iv.rule.Name, image, status)
}

func (iv *imageVerifier) verifyImage(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	cfg config.Configuration,
) (*engineapi.RuleResponse, string) {
	if len(imageVerify.Attestors) <= 0 && len(imageVerify.Attestations) <= 0 {
		return nil, ""
	}
	image := imageInfo.String()
	for _, att := range imageVerify.Attestations {
		if att.Type == "" && att.PredicateType != "" {
			att.Type = att.PredicateType
		}
	}
	iv.logger.V(2).Info("verifying image signatures", "image", image, "attestors", len(imageVerify.Attestors), "attestations", len(imageVerify.Attestations))
	if err := iv.policyContext.JSONContext().AddImageInfo(imageInfo, cfg); err != nil {
		iv.logger.Error(err, "failed to add image to context", "image", image)
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("failed to add image to context %s", image), err, iv.rule.ReportProperties), ""
	}
	if !matchReferences(imageVerify.ImageReferences, image) {
		return engineapi.RuleSkip(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("skipping image reference image %s, policy %s ruleName %s", image, iv.policyContext.Policy().GetName(), iv.rule.Name), iv.rule.ReportProperties), ""
	}

	if matchReferences(imageVerify.SkipImageReferences, image) {
		iv.logger.V(3).Info("skipping image reference", "image", image, "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name)
		iv.ivm.Add(image, engineapi.ImageVerificationSkip)
		return engineapi.RuleSkip(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("skipping image reference image %s, policy %s ruleName %s", image, iv.policyContext.Policy().GetName(), iv.rule.Name), iv.rule.ReportProperties).WithEmitWarning(true), ""
	}
	if len(imageVerify.Attestors) > 0 {
		ruleResp, cosignResp := iv.verifyAttestors(ctx, imageVerify.Attestors, imageVerify, imageInfo)
		if ruleResp.Status() != engineapi.RuleStatusPass {
			return ruleResp, ""
		}
		if imageInfo.Digest == "" {
			imageInfo.Digest = cosignResp.Digest
		}
		if len(imageVerify.Attestations) == 0 {
			// The rule's validate.deny gates live past verifyAttestations,
			// which an entry with no attestations never reaches. Run them
			// here or a deny on such an entry is accepted and never applied.
			if resp, handled := iv.runValidationGates(ctx); handled {
				return resp, cosignResp.Digest
			}
			return ruleResp, cosignResp.Digest
		}
	}

	return iv.verifyAttestations(ctx, imageVerify, imageInfo)
}

func (iv *imageVerifier) verifyAttestors(
	ctx context.Context,
	attestors []kyvernov1.AttestorSet,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
) (*engineapi.RuleResponse, *verifiers.Response) {
	var cosignResponse *verifiers.Response
	image := imageInfo.String()
	for i, attestorSet := range attestors {
		var err error
		path := fmt.Sprintf(".attestors[%d]", i)
		iv.logger.V(4).Info("verifying attestors", "path", path)
		cosignResponse, err = iv.verifyAttestorSet(ctx, attestorSet, imageVerify, imageInfo, path)
		if err != nil {
			iv.logger.Error(err, "failed to verify image", "image", image)
			return iv.handleRegistryErrors(image, err), nil
		}
	}
	if cosignResponse == nil {
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, "invalid response", fmt.Errorf("nil"), iv.rule.ReportProperties), nil
	}
	msg := fmt.Sprintf("verified image signatures for %s", image)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), cosignResponse
}

// isInfrastructureError reports whether err reflects an inability to reach the
// registry at all. Registry HTTP responses are deliberately excluded: Error
// only blocks under failurePolicy Fail, so treating an auth or 5xx response as
// Error would admit the image under failurePolicy Ignore.
func isInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

// handle registry network errors as a rule error (instead of a policy failure)
func (iv *imageVerifier) handleRegistryErrors(image string, err error) *engineapi.RuleResponse {
	msg := fmt.Sprintf("failed to verify image %s: %s", image, err.Error())
	if isInfrastructureError(err) {
		return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, fmt.Sprintf("failed to verify image %s", image), err, iv.rule.ReportProperties)
	}
	return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties)
}

func (iv *imageVerifier) verifyAttestations(
	ctx context.Context,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
) (*engineapi.RuleResponse, string) {
	image := imageInfo.String()
	for i, attestation := range imageVerify.Attestations {
		var errorList []error

		path := fmt.Sprintf(".attestations[%d]", i)

		iv.logger.V(2).Info(fmt.Sprintf("attestation %+v", attestation))
		if attestation.Type == "" && attestation.PredicateType == "" {
			return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, path+": missing type", iv.rule.ReportProperties), ""
		}

		if attestation.Type == "" && attestation.PredicateType != "" {
			attestation.Type = attestation.PredicateType
		}

		if len(attestation.Attestors) == 0 {
			// add an empty attestor to allow fetching and checking attestations
			attestation.Attestors = []kyvernov1.AttestorSet{{Entries: []kyvernov1.Attestor{{}}}}
		}

		for j, attestor := range attestation.Attestors {
			attestorPath := fmt.Sprintf("%s.attestors[%d]", path, j)
			requiredCount := attestor.RequiredCount()
			verifiedCount := 0
			// errorList spans every attestor set of this attestation, but the
			// Error-vs-Fail decision below belongs to the set that failed: an
			// infrastructure error recorded by an earlier, satisfied set must
			// not turn this set's genuine rejection into an Error, which
			// failurePolicy Ignore admits.
			setErrorsFrom := len(errorList)

			for _, a := range attestor.Entries {
				var attestationError error
				entryPath := fmt.Sprintf("%s.entries[%d]", attestorPath, i)
				v, opts, subPath := iv.buildVerifier(a, imageVerify, image, &imageVerify.Attestations[i])
				cosignResp, err := v.FetchAttestations(ctx, *opts)
				if err != nil {
					iv.logger.Error(err, "failed to fetch attestations", "image", image)
					errorList = append(errorList, err)
					continue
				}

				name := imageVerify.Attestations[i].Name

				rawResp, err := getRawResp(cosignResp.Statements)
				if err != nil {
					iv.logger.Error(err, "Error while finding report in statement")
					errorList = append(errorList, err)
					continue
				}

				err = iv.policyContext.JSONContext().AddContextEntry(name, rawResp)
				if err != nil {
					iv.logger.Error(err, "failed to add resource data to context entry")
					errorList = append(errorList, err)
					continue
				}

				if imageInfo.Digest == "" {
					imageInfo.Digest = cosignResp.Digest
					image = imageInfo.String()
				}

				attestationError = iv.verifyAttestation(cosignResp.Statements, attestation, imageInfo)

				if attestationError == nil {
					verifiedCount++
					if verifiedCount >= requiredCount {
						iv.logger.V(2).Info("image attestations verification succeeded", "image", image, "verifiedCount", verifiedCount, "requiredCount", requiredCount)
						break
					}
				} else {
					attestationError = fmt.Errorf("%s: %w", entryPath+subPath, attestationError)
					iv.logger.Error(attestationError, "image attestation verification failed", "image", image)
					errorList = append(errorList, attestationError)
				}
			}

			err := multierr.Combine(errorList...)
			errMsg := "attestations verification failed"
			if err != nil {
				errMsg = err.Error()
			}
			if verifiedCount < requiredCount {
				msg := fmt.Sprintf("image attestations verification failed, verifiedCount: %v, requiredCount: %v, error: %s", verifiedCount, requiredCount, errMsg)
				// A fetch failure caused by an unreachable registry or a
				// rejected credential means the constraint was never
				// actually checked; report that distinctly from a genuine
				// rejection.
				if e := firstInfrastructureError(errorList[setErrorsFrom:]); e != nil {
					return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, msg, e, iv.rule.ReportProperties), ""
				}
				return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), ""
			}
		}

		iv.logger.V(4).Info("attestation checks passed", "path", path, "image", imageInfo.String(), "type", attestation.Type)
	}

	if resp, handled := iv.runValidationGates(ctx); handled {
		return resp, imageInfo.Digest
	}

	msg := fmt.Sprintf("verified image attestations for %s", image)
	iv.logger.V(2).Info(msg)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), imageInfo.Digest
}

// runValidationGates evaluates validate.deny for every entry of the rule, not
// just the one being verified: a deny is declared on an entry but applies to
// the rule. Reports whether the rule declares any validation at all, so a
// caller can fall back to its own response when it does not.
func (iv *imageVerifier) runValidationGates(ctx context.Context) (*engineapi.RuleResponse, bool) {
	if !iv.rule.HasValidateImageVerification() {
		return nil, false
	}
	for _, imageVerify := range iv.rule.VerifyImages {
		if err := iv.validate(imageVerify, ctx); err != nil {
			msg := fmt.Sprintf("validation in verifyImages failed: %v", err)
			iv.logger.Error(err, "validation in verifyImages failed")
			return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), true
		}
	}
	msg := fmt.Sprintf("verifyImages validation is passed in %v rule", iv.rule.Name)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), true
}

func (iv *imageVerifier) verifyAttestorSet(
	ctx context.Context,
	attestorSet kyvernov1.AttestorSet,
	imageVerify kyvernov1.ImageVerification,
	imageInfo apiutils.ImageInfo,
	path string,
) (*verifiers.Response, error) {
	var errorList []error
	verifiedCount := 0
	attestorSet = ExpandStaticKeys(attestorSet)
	requiredCount := attestorSet.RequiredCount()
	image := imageInfo.String()

	for i, a := range attestorSet.Entries {
		var entryError error
		var cosignResp *verifiers.Response
		attestorPath := fmt.Sprintf("%s.entries[%d]", path, i)
		iv.logger.V(4).Info("verifying attestorSet", "path", attestorPath, "image", image)

		if a.Attestor != nil {
			nestedAttestorSet, err := kyvernov1.AttestorSetUnmarshal(a.Attestor)
			if err != nil {
				entryError = fmt.Errorf("failed to unmarshal nested attestor %s: %w", attestorPath, err)
			} else {
				attestorPath += ".attestor"
				cosignResp, entryError = iv.verifyAttestorSet(ctx, *nestedAttestorSet, imageVerify, imageInfo, attestorPath)
			}
		} else {
			v, opts, subPath := iv.buildVerifier(a, imageVerify, image, nil)
			cosignResp, entryError = v.VerifySignature(ctx, *opts)
			if entryError != nil {
				entryError = fmt.Errorf("%s: %w", attestorPath+subPath, entryError)
			}
		}

		if entryError == nil {
			verifiedCount++
			if verifiedCount >= requiredCount {
				iv.logger.V(2).Info("image attestors verification succeeded", "image", image, "verifiedCount", verifiedCount, "requiredCount", requiredCount)
				return cosignResp, nil
			}
		} else {
			errorList = append(errorList, entryError)
		}
	}

	err := multierr.Combine(errorList...)
	iv.logger.Info("image attestors verification failed", "image", image, "verifiedCount", verifiedCount, "requiredCount", requiredCount, "errors", err.Error())
	return nil, err
}

// firstInfrastructureError returns the first error that means the constraint
// was never actually checked, rather than checked and rejected. Callers must
// pass only the errors of the attestor set they are deciding: an error from a
// different, already satisfied set would turn this set's genuine rejection
// into an Error, which failurePolicy Ignore admits.
func firstInfrastructureError(errs []error) error {
	for _, e := range errs {
		if isInfrastructureError(e) {
			return e
		}
	}
	return nil
}

func (iv *imageVerifier) buildVerifier(
	attestor kyvernov1.Attestor,
	imageVerify kyvernov1.ImageVerification,
	image string,
	attestation *kyvernov1.Attestation,
) (verifiers.ImageVerifier, *verifiers.Options, string) {
	switch imageVerify.Type {
	case kyvernov1.Notary:
		return iv.buildNotaryVerifier(attestor, image, attestation)
	default:
		return iv.buildCosignVerifier(attestor, imageVerify, image, attestation)
	}
}

func (iv *imageVerifier) buildCosignVerifier(
	attestor kyvernov1.Attestor,
	imageVerify kyvernov1.ImageVerification,
	image string,
	attestation *kyvernov1.Attestation,
) (verifiers.ImageVerifier, *verifiers.Options, string) {
	path := ""
	opts := &verifiers.Options{
		ImageRef:           image,
		Repository:         imageVerify.Repository,
		CosignOCI11:        imageVerify.CosignOCI11,
		Annotations:        imageVerify.Annotations,
		SignatureAlgorithm: attestor.SignatureAlgorithm,
		Client:             iv.rclient,
	}

	if imageVerify.Type == kyvernov1.SigstoreBundle {
		opts.SigstoreBundle = true
	}

	if imageVerify.Roots != "" {
		opts.Roots = imageVerify.Roots
	}

	if attestation != nil {
		opts.PredicateType = attestation.PredicateType
		opts.Type = attestation.Type
		opts.IgnoreSCT = true // TODO: Add option to allow SCT when attestors are not provided
		if attestation.PredicateType != "" && attestation.Type == "" {
			iv.logger.V(4).Info("predicate type has been deprecated, please use type instead", "image", image)
			opts.Type = attestation.PredicateType
		}
		opts.FetchAttestations = true
	}

	if attestor.Keys != nil {
		path = path + ".keys"
		if attestor.Keys.PublicKeys != "" {
			opts.Key = attestor.Keys.PublicKeys
		} else if attestor.Keys.Secret != nil {
			opts.Key = fmt.Sprintf("k8s://%s/%s", attestor.Keys.Secret.Namespace, attestor.Keys.Secret.Name)
		} else if attestor.Keys.KMS != "" {
			opts.Key = attestor.Keys.KMS
		}
		if attestor.Keys.Rekor != nil {
			opts.RekorURL = attestor.Keys.Rekor.URL
			opts.RekorPubKey = attestor.Keys.Rekor.RekorPubKey
			opts.IgnoreTlog = attestor.Keys.Rekor.IgnoreTlog
		} else {
			opts.RekorURL = "https://rekor.sigstore.dev"
			opts.IgnoreTlog = false
		}

		if attestor.Keys.CTLog != nil {
			opts.IgnoreSCT = attestor.Keys.CTLog.IgnoreSCT
			opts.CTLogsPubKey = attestor.Keys.CTLog.CTLogPubKey
			opts.TSACertChain = attestor.Keys.CTLog.TSACertChain
		} else {
			opts.IgnoreSCT = false
		}

		opts.SignatureAlgorithm = attestor.Keys.SignatureAlgorithm
	} else if attestor.Certificates != nil {
		path = path + ".certificates"
		opts.Cert = attestor.Certificates.Certificate
		opts.CertChain = attestor.Certificates.CertificateChain
		if attestor.Certificates.Rekor != nil {
			opts.RekorURL = attestor.Certificates.Rekor.URL
			opts.RekorPubKey = attestor.Certificates.Rekor.RekorPubKey
			opts.IgnoreTlog = attestor.Certificates.Rekor.IgnoreTlog
		} else {
			opts.RekorURL = "https://rekor.sigstore.dev"
			opts.IgnoreTlog = false
		}

		if attestor.Certificates.CTLog != nil {
			opts.IgnoreSCT = attestor.Certificates.CTLog.IgnoreSCT
			opts.CTLogsPubKey = attestor.Certificates.CTLog.CTLogPubKey
			opts.TSACertChain = attestor.Certificates.CTLog.TSACertChain
		} else {
			opts.IgnoreSCT = false
		}
	} else if attestor.Keyless != nil {
		path = path + ".keyless"
		if attestor.Keyless.Rekor != nil {
			opts.RekorURL = attestor.Keyless.Rekor.URL
			opts.RekorPubKey = attestor.Keyless.Rekor.RekorPubKey
			opts.IgnoreTlog = attestor.Keyless.Rekor.IgnoreTlog
		} else {
			opts.RekorURL = "https://rekor.sigstore.dev"
			opts.IgnoreTlog = false
		}

		if attestor.Keyless.CTLog != nil {
			opts.IgnoreSCT = attestor.Keyless.CTLog.IgnoreSCT
			opts.CTLogsPubKey = attestor.Keyless.CTLog.CTLogPubKey
			opts.TSACertChain = attestor.Keyless.CTLog.TSACertChain
		} else {
			opts.IgnoreSCT = false
		}

		opts.Roots = attestor.Keyless.Roots
		opts.Issuer = attestor.Keyless.Issuer
		opts.IssuerRegExp = attestor.Keyless.IssuerRegExp
		opts.Subject = attestor.Keyless.Subject
		opts.SubjectRegExp = attestor.Keyless.SubjectRegExp
		opts.AdditionalExtensions = attestor.Keyless.AdditionalExtensions
	}

	if attestor.Repository != "" {
		opts.Repository = attestor.Repository
	}

	if attestor.Annotations != nil {
		opts.Annotations = attestor.Annotations
	}

	iv.logger.V(4).Info("cosign verifier built", "ignoreTlog", opts.IgnoreTlog, "ignoreSCT", opts.IgnoreSCT, "image", image)
	return cosign.NewVerifier(), opts, path
}

func (iv *imageVerifier) buildNotaryVerifier(
	attestor kyvernov1.Attestor,
	image string,
	attestation *kyvernov1.Attestation,
) (verifiers.ImageVerifier, *verifiers.Options, string) {
	path := ""
	opts := &verifiers.Options{
		ImageRef:  image,
		Cert:      attestor.Certificates.Certificate,
		CertChain: attestor.Certificates.CertificateChain,
		Client:    iv.rclient,
	}

	if attestation != nil {
		opts.Type = attestation.Type
		opts.PredicateType = attestation.PredicateType
		if attestation.PredicateType != "" && attestation.Type == "" {
			iv.logger.V(2).Info("predicate type has been deprecated, please use type instead", "image", image)
			opts.Type = attestation.PredicateType
		}
		opts.FetchAttestations = true
	}

	if attestor.Repository != "" {
		opts.Repository = attestor.Repository
	}

	if attestor.Annotations != nil {
		opts.Annotations = attestor.Annotations
	}

	return notary.NewVerifier(), opts, path
}

func (iv *imageVerifier) verifyAttestation(statements []map[string]any, attestation kyvernov1.Attestation, imageInfo apiutils.ImageInfo) error {
	if attestation.Type == "" && attestation.PredicateType == "" {
		return fmt.Errorf("a type is required")
	}
	image := imageInfo.String()
	statementsByPredicate, types, err := buildStatementMap(statements)
	if err != nil {
		return fmt.Errorf("failed to process attestation statements: %w", err)
	}
	iv.logger.V(4).Info("checking attestations", "predicates", types, "image", image)
	statements = statementsByPredicate[attestation.Type]
	if statements == nil {
		iv.logger.V(2).Info("no attestations found for predicate", "type", attestation.Type, "predicates", types, "image", imageInfo.String())
		return fmt.Errorf("attestions not found for predicate type %s", attestation.Type)
	}
	for _, s := range statements {
		iv.logger.V(3).Info("checking attestation", "predicates", types, "image", imageInfo.String())
		val, msg, err := iv.checkAttestations(attestation, s)
		if err != nil {
			return fmt.Errorf("failed to check attestations: %w", err)
		}
		if !val {
			return fmt.Errorf("attestation checks failed for %s and predicate %s: %s", imageInfo.String(), attestation.Type, msg)
		}
	}
	return nil
}

func (iv *imageVerifier) checkAttestations(a kyvernov1.Attestation, s map[string]any) (bool, string, error) {
	if len(a.Conditions) == 0 {
		return true, "", nil
	}
	iv.policyContext.JSONContext().Checkpoint()
	defer iv.policyContext.JSONContext().Restore()
	return EvaluateConditions(a.Conditions, iv.policyContext.JSONContext(), s, iv.logger)
}

func (iv *imageVerifier) handleMutateDigest(ctx context.Context, digest string, imageInfo apiutils.ImageInfo) (*jsonpatch.JsonPatchOperation, string, error) {
	if imageInfo.Digest != "" {
		return nil, "", nil
	}
	if digest == "" {
		desc, err := iv.rclient.FetchImageDescriptor(ctx, imageInfo.String())
		if err != nil {
			return nil, "", err
		}
		digest = desc.Digest.String()
	}
	patch := makeAddDigestPatch(imageInfo, digest)
	iv.logger.V(4).Info("adding digest patch", "image", imageInfo.String(), "patch", patch.Json())
	return &patch, digest, nil
}

func (iv *imageVerifier) validate(imageVerify kyvernov1.ImageVerification, ctx context.Context) error {
	spec := iv.policyContext.Policy().GetSpec()
	background := spec.BackgroundProcessingEnabled()
	err := policy.ValidateVariables(iv.policyContext.Policy(), background)
	if err != nil {
		return err
	}

	if imageVerify.Validation.Deny != nil {
		if err := iv.validateDeny(imageVerify); err != nil {
			return err
		}
	}
	return nil
}

func (iv *imageVerifier) validateDeny(imageVerify kyvernov1.ImageVerification) error {
	if deny, msg, err := CheckDenyPreconditions(iv.logger, iv.policyContext.JSONContext(), imageVerify.Validation.Deny.GetAnyAllConditions()); err != nil {
		return fmt.Errorf("failed to check deny conditions: %v", err)
	} else {
		if deny {
			return fmt.Errorf("%s", iv.getDenyMessage(imageVerify, deny, msg))
		}
		return nil
	}
}

func (iv *imageVerifier) getDenyMessage(imageVerify kyvernov1.ImageVerification, deny bool, msg string) string {
	if !deny {
		return fmt.Sprintf("validation imageVerify '%s' passed.", imageVerify.Validation.Message)
	}

	if imageVerify.Validation.Message == "" && msg == "" {
		return fmt.Sprintf("validation error: imageVerify %s failed", imageVerify.Validation.Message)
	}

	s := stringutils.JoinNonEmpty([]string{imageVerify.Validation.Message, msg}, "; ")
	raw, err := variables.SubstituteAll(iv.logger, iv.policyContext.JSONContext(), s)
	if err != nil {
		return msg
	}

	switch typed := raw.(type) {
	case string:
		return typed
	default:
		return "the produced message didn't resolve to a string, check your policy definition."
	}
}

func (iv *imageVerifier) getDeclaredContextNames() map[string]bool {
	names := make(map[string]bool)
	for _, ce := range iv.origRule.Context {
		if ce.Name != "" {
			names[ce.Name] = true
		}
	}
	for _, ce := range iv.rule.Context {
		if ce.Name != "" {
			names[ce.Name] = true
		}
	}
	return names
}

func (iv *imageVerifier) evaluateRuleValidation(ctx context.Context) error {
	if !iv.rule.HasValidateImageVerification() {
		return nil
	}
	for _, imageVerify := range iv.rule.VerifyImages {
		if err := iv.validate(imageVerify, ctx); err != nil {
			return err
		}
	}
	return nil
}

func getAttestationStatementNames(iv *kyvernov1.ImageVerification) map[string]bool {
	names := make(map[string]bool)
	if iv == nil {
		return names
	}
	for _, att := range iv.Attestations {
		if att.Name != "" {
			names[att.Name] = true
		}
	}
	return names
}

func isStatementScopedVariable(varName string, statementNames map[string]bool, declaredContextNames map[string]bool) bool {
	v := strings.TrimSpace(varName)
	if v == "@" || strings.HasPrefix(v, "@.") || strings.HasPrefix(v, "@[") || strings.Contains(v, "(@") || strings.Contains(v, " @") {
		return true
	}
	if v == "predicate" || strings.HasPrefix(v, "predicate.") || strings.HasPrefix(v, "predicate[") || strings.Contains(v, "predicate.") || strings.Contains(v, "predicate[") {
		return true
	}
	for name := range statementNames {
		if name == "" {
			continue
		}
		if v == name || strings.HasPrefix(v, name+".") || strings.HasPrefix(v, name+"[") || strings.Contains(v, name+".") || strings.Contains(v, name+"[") {
			return true
		}
	}
	root := v
	if idx := strings.IndexAny(root, ".[("); idx != -1 {
		root = root[:idx]
	}
	root = strings.TrimSpace(root)
	if declaredContextNames != nil && declaredContextNames[root] {
		return false
	}
	return true
}

func bindResolvedConditions(iv *kyvernov1.ImageVerification, ctx enginecontext.EvalInterface, declaredNames map[string]bool) {
	if iv == nil || ctx == nil {
		return
	}
	statementNames := getAttestationStatementNames(iv)
	for i := range iv.Attestations {
		for j := range iv.Attestations[i].Conditions {
			bindAnyAllConditions(&iv.Attestations[i].Conditions[j], ctx, statementNames, declaredNames)
		}
	}
	if iv.Validation.Deny != nil && iv.Validation.Deny.GetAnyAllConditions() != nil {
		if condBytes, err := json.Marshal(iv.Validation.Deny.GetAnyAllConditions()); err == nil && len(condBytes) > 0 {
			var denyObj interface{}
			if err := json.Unmarshal(condBytes, &denyObj); err == nil {
				newDenyObj := substituteContextVariables(denyObj, ctx, statementNames, declaredNames)
				iv.Validation.Deny.SetAnyAllConditions(newDenyObj)
			}
		}
	}
}

func bindAnyAllConditions(cond *kyvernov1.AnyAllConditions, ctx enginecontext.EvalInterface, statementNames map[string]bool, declaredNames map[string]bool) {
	for k := range cond.AllConditions {
		bindCondition(&cond.AllConditions[k], ctx, statementNames, declaredNames)
	}
	for k := range cond.AnyConditions {
		bindCondition(&cond.AnyConditions[k], ctx, statementNames, declaredNames)
	}
}

func bindCondition(c *kyvernov1.Condition, ctx enginecontext.EvalInterface, statementNames map[string]bool, declaredNames map[string]bool) {
	if c.RawKey != nil && len(c.RawKey.Raw) > 0 {
		var keyObj interface{}
		if err := json.Unmarshal(c.RawKey.Raw, &keyObj); err == nil {
			newKey := substituteContextVariables(keyObj, ctx, statementNames, declaredNames)
			if newRaw, err := json.Marshal(newKey); err == nil {
				c.RawKey = &apiextv1.JSON{Raw: newRaw}
			}
		}
	}
	if c.RawValue != nil && len(c.RawValue.Raw) > 0 {
		var valObj interface{}
		if err := json.Unmarshal(c.RawValue.Raw, &valObj); err == nil {
			newVal := substituteContextVariables(valObj, ctx, statementNames, declaredNames)
			if newRaw, err := json.Marshal(newVal); err == nil {
				c.RawValue = &apiextv1.JSON{Raw: newRaw}
			}
		}
	}
}

func substituteContextVariables(data interface{}, ctx enginecontext.EvalInterface, statementNames map[string]bool, declaredNames map[string]bool) interface{} {
	switch v := data.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") && !strings.Contains(trimmed[2:len(trimmed)-2], "{{") {
			varName := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			if isStatementScopedVariable(varName, statementNames, declaredNames) {
				return v
			}
			if res, err := ctx.Query(varName); err == nil && res != nil {
				return res
			}
		}
		return substituteContextVariablesInString(v, ctx, statementNames, declaredNames)
	case []interface{}:
		res := make([]interface{}, len(v))
		for i, item := range v {
			res[i] = substituteContextVariables(item, ctx, statementNames, declaredNames)
		}
		return res
	case map[string]interface{}:
		res := make(map[string]interface{}, len(v))
		for k, val := range v {
			res[k] = substituteContextVariables(val, ctx, statementNames, declaredNames)
		}
		return res
	default:
		return data
	}
}

func substituteContextVariablesInString(val string, ctx enginecontext.EvalInterface, statementNames map[string]bool, declaredNames map[string]bool) string {
	return variables.ReplaceAllVars(val, func(s string) string {
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
			varName := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
			if isStatementScopedVariable(varName, statementNames, declaredNames) {
				return s
			}
			if res, err := ctx.Query(varName); err == nil && res != nil {
				if b, err := json.Marshal(res); err == nil {
					if str, ok := res.(string); ok {
						return str
					}
					return string(b)
				}
				return fmt.Sprintf("%#v", res)
			}
		}
		return s
	})
}

func attestorHasUnbindableKeyMaterial(attestor kyvernov1.Attestor) bool {
	if attestor.Keys != nil {
		if attestor.Keys.KMS != "" || attestor.Keys.Secret != nil {
			return true
		}
		if pk := strings.TrimSpace(attestor.Keys.PublicKeys); pk != "" && !strings.HasPrefix(pk, "-----BEGIN PUBLIC KEY-----") {
			return true
		}
	}
	if attestor.Attestor != nil && len(attestor.Attestor.Raw) > 0 {
		if as, err := kyvernov1.AttestorSetUnmarshal(attestor.Attestor); err == nil && as != nil {
			for _, entry := range as.Entries {
				if attestorHasUnbindableKeyMaterial(entry) {
					return true
				}
			}
		}
	}
	return false
}

func attestationHasUnbindableMaterial(att kyvernov1.Attestation) bool {
	if b, err := json.Marshal(att.Conditions); err == nil && (strings.Contains(string(b), "k8s://") || strings.Contains(string(b), "$(")) {
		return true
	}
	return false
}

func hasUnbindableKeyMaterial(imageVerify *kyvernov1.ImageVerification) bool {
	if imageVerify == nil {
		return false
	}
	for _, as := range imageVerify.Attestors {
		for _, entry := range as.Entries {
			if attestorHasUnbindableKeyMaterial(entry) {
				return true
			}
		}
	}
	for _, att := range imageVerify.Attestations {
		for _, as := range att.Attestors {
			for _, entry := range as.Entries {
			if attestorHasUnbindableKeyMaterial(entry) {
				return true
			}
			}
		}
		if attestationHasUnbindableMaterial(att) {
			return true
		}
	}
	if imageVerify.Validation.Deny != nil && imageVerify.Validation.Deny.GetAnyAllConditions() != nil {
		if b, err := json.Marshal(imageVerify.Validation.Deny.GetAnyAllConditions()); err == nil && (strings.Contains(string(b), "k8s://") || strings.Contains(string(b), "$(")) {
			return true
		}
	}
	return false
}

func hasUnresolvedConditionVariables(imageVerify *kyvernov1.ImageVerification, opts ...any) bool {
	if imageVerify == nil {
		return false
	}
	var evalCtx enginecontext.EvalInterface
	var declaredNames map[string]bool
	for _, opt := range opts {
		switch v := opt.(type) {
		case enginecontext.EvalInterface:
			evalCtx = v
		case map[string]bool:
			declaredNames = v
		}
	}
	ivCopy := imageVerify.DeepCopy()
	if evalCtx != nil {
		bindResolvedConditions(ivCopy, evalCtx, declaredNames)
	}
	for _, att := range ivCopy.Attestations {
		if len(att.Conditions) > 0 {
			if condBytes, err := json.Marshal(att.Conditions); err == nil {
				if strings.Contains(string(condBytes), "{{") || strings.Contains(string(condBytes), "$(") || strings.Contains(string(condBytes), "k8s://") {
					return true
				}
			}
		}
	}
	if ivCopy.Validation.Deny != nil && ivCopy.Validation.Deny.GetAnyAllConditions() != nil {
		if condBytes, err := json.Marshal(ivCopy.Validation.Deny.GetAnyAllConditions()); err == nil {
			if strings.Contains(string(condBytes), "{{") || strings.Contains(string(condBytes), "$(") || strings.Contains(string(condBytes), "k8s://") {
				return true
			}
		}
	}
	return false
}

func hasDynamicAuthority(origRule, rule *kyvernov1.Rule, imageVerify *kyvernov1.ImageVerification) bool {
	if origRule != nil {
		if len(origRule.Context) > 0 {
			return true
		}
		if ruleBytes, err := json.Marshal(origRule); err == nil && (strings.Contains(string(ruleBytes), "{{") || strings.Contains(string(ruleBytes), "$(")) {
			return true
		}
	}
	if rule != nil {
		if len(rule.Context) > 0 {
			return true
		}
		if ruleBytes, err := json.Marshal(rule); err == nil && (strings.Contains(string(ruleBytes), "{{") || strings.Contains(string(ruleBytes), "$(")) {
			return true
		}
	}
	if imageVerify != nil && (hasUnbindableKeyMaterial(imageVerify) || hasUnresolvedConditionVariables(imageVerify)) {
		return true
	}
	if imageVerify != nil {
		if ivBytes, err := json.Marshal(imageVerify); err == nil && (strings.Contains(string(ivBytes), "{{") || strings.Contains(string(ivBytes), "$(")) {
			return true
		}
	}
	return false
}

func (iv *imageVerifier) hasDynamicAuthority(imageVerify *kyvernov1.ImageVerification) bool {
	return hasDynamicAuthority(&iv.origRule, &iv.rule, imageVerify)
}
