package internal

import (
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
	ivm           *engineapi.ImageVerificationMetadata
}

func NewImageVerifier(
	logger logr.Logger,
	rclient engineapi.RegistryClient,
	ivCache imageverifycache.Client,
	policyContext engineapi.PolicyContext,
	rule kyvernov1.Rule,
	ivm *engineapi.ImageVerificationMetadata,
) ImageVerifier {
	return &imageVerifier{
		logger:        logger,
		rclient:       rclient,
		ivCache:       ivCache,
		policyContext: policyContext,
		rule:          rule,
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

	// rule.VerifyImages is a slice and every entry verifies under the same
	// rule name, so the cache scope must also bind what this entry requires.
	// Without it a permissive entry's pass is served to a stricter entry in
	// the same rule, within a single admission.
	cacheScope, err := VerifyImagesCacheScope(iv.rule.Name, imageVerify)
	useCache := imageVerify.UseCache && err == nil
	if err != nil {
		iv.logger.Error(err, "failed to derive cache scope, caching disabled for this rule entry", "ruleName", iv.rule.Name)
	}
	// A hit returns a pass without running verifyImage, which is where the
	// gates that read the request live. Those depend on the resource, not on
	// this entry, so no key can separate two resources that differ only in
	// what the gate would say. Verify every time instead.
	if useCache && hasResourceDependentGate(iv.rule, imageVerify) {
		iv.logger.V(2).Info("caching disabled: entry declares a gate evaluated against the request", "ruleName", iv.rule.Name)
		useCache = false
	}
	if useCache && hasUnbindableKeyMaterial(imageVerify) {
		iv.logger.V(2).Info("caching disabled: entry verifies with key material the cache key cannot bind", "ruleName", iv.rule.Name)
		useCache = false
	}

	for _, imageInfo := range matchedImageInfos {
		image := imageInfo.String()

		// Cache keys must be bound to a resolved digest, never a tag: a tag
		// is a registry-movable name, and a verdict keyed on it would be
		// trusted for whatever the tag points to later, not what it pointed
		// to when verified. If the reference isn't already digest-pinned,
		// resolve it now, before the cache lookup. This HEAD is the binding
		// between the mutable name and the immutable verdict, not overhead
		// to optimize away: skipping it would let a moved tag serve a stale
		// cache hit.
		// verifyImage matches imageReferences and skipImageReferences below,
		// after the lookup, and the cache key is digest-normalised, so a pass
		// cached for the digest form would be served for a tag this entry
		// means to skip or does not match at all. Decide that here instead.
		imageUseCache := useCache &&
			matchReferences(imageVerify.ImageReferences, image) &&
			!matchReferences(imageVerify.SkipImageReferences, image)
		if useCache && !imageUseCache {
			iv.logger.V(2).Info("caching disabled: entry skips or does not match this image reference", "image", image, "ruleName", iv.rule.Name)
		}

		cacheInfo := imageInfo
		if cacheInfo.Digest == "" && iv.ivCache != nil && imageUseCache {
			if desc, err := iv.rclient.FetchImageDescriptor(ctx, image); err == nil {
				cacheInfo.Digest = desc.Digest.String()
			} else {
				iv.logger.V(4).Info("failed to resolve image digest for cache lookup", "image", image, "error", err.Error())
			}
		}
		cacheKey := cacheInfo.String()

		isInCache := false
		if iv.ivCache != nil {
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
			ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "verified from cache", iv.rule.ReportProperties)
			digest = cacheInfo.Digest
		} else {
			iv.logger.V(2).Info("cache entry not found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
			ruleResp, digest = iv.verifyImage(ctx, imageVerify, imageInfo, cfg)
			if ruleResp != nil && ruleResp.Status() == engineapi.RuleStatusPass {
				if iv.ivCache != nil {
					// Key symmetry is mandatory: the Set key must be derived
					// the same way as the Get key above (ImageInfo.String()
					// with Digest populated), or a hit here becomes
					// unreachable from the validating side and every "hit"
					// silently degrades into a full re-verification.
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
			}
			responses = append(responses, ruleResp)
		}
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
func hasUnbindableKeyMaterial(imageVerify kyvernov1.ImageVerification) bool {
	for _, attestorSet := range imageVerify.Attestors {
		if attestorSetHasUnbindableKeyMaterial(attestorSet) {
			return true
		}
	}
	for _, attestation := range imageVerify.Attestations {
		for _, attestorSet := range attestation.Attestors {
			if attestorSetHasUnbindableKeyMaterial(attestorSet) {
				return true
			}
		}
	}
	return false
}

func attestorSetHasUnbindableKeyMaterial(attestorSet kyvernov1.AttestorSet) bool {
	for _, entry := range attestorSet.Entries {
		// Walk nested attestor sets the same way verifyAttestorSet does, or a
		// reference one level down is verified against but never inspected here.
		if entry.Attestor != nil {
			nested, err := kyvernov1.AttestorSetUnmarshal(entry.Attestor)
			if err != nil {
				// Unreadable here but verifyAttestorSet will try to read it too.
				// Treat it as unbindable rather than assume it holds nothing.
				return true
			}
			if attestorSetHasUnbindableKeyMaterial(*nested) {
				return true
			}
			continue
		}
		if entry.Keys == nil {
			continue
		}
		if entry.Keys.Secret != nil || entry.Keys.KMS != "" {
			return true
		}
		// An inline key carries its own material; anything else (k8s://, a KMS
		// url) only names it. Use the same test the verifier uses to decide
		// whether to treat this as inline, or a KMS alias that merely contains
		// the PEM header would be loaded as a reference and keyed as inline.
		if entry.Keys.PublicKeys != "" &&
			!strings.HasPrefix(strings.TrimSpace(entry.Keys.PublicKeys), "-----BEGIN PUBLIC KEY-----") {
			return true
		}
	}
	return false
}

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
func VerifyImagesCacheScope(ruleName string, imageVerify kyvernov1.ImageVerification) (string, error) {
	// Bind everything by default and subtract only what must not participate,
	// so a newly added verification field is bound without anyone remembering
	// to update this list.
	scoped := *imageVerify.DeepCopy()
	// Not a verification input, and MutateDigest can differ between the
	// mutating and validating stages; binding either would break key symmetry
	// and silently disable the cache rather than scope it.
	scoped.MutateDigest = false
	scoped.UseCache = false

	encoded, err := json.Marshal(scoped)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return ruleName + ";" + hex.EncodeToString(sum[:]), nil
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
				for _, e := range errorList {
					if isInfrastructureError(e) {
						return engineapi.RuleError(iv.rule.Name, engineapi.ImageVerify, msg, e, iv.rule.ReportProperties), ""
					}
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
