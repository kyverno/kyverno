package internal

import (
	"context"
	"errors"
	"fmt"
	"net"

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
	"github.com/kyverno/kyverno/pkg/utils/jsonpointer"
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

	for _, imageInfo := range matchedImageInfos {
		image := imageInfo.String()

		pointer := jsonpointer.ParsePath(imageInfo.Pointer).JMESPath()
		changed, err := iv.policyContext.JSONContext().HasChanged(pointer)
		if err == nil && !changed {
			iv.logger.V(4).Info("no change in image, skipping check", "image", image)
			iv.ivm.Add(image, engineapi.ImageVerificationPass)
			continue
		}

		isInCache := false
		if iv.ivCache != nil {
			found, err := iv.ivCache.Get(ctx, iv.policyContext.Policy(), iv.rule.Name, image, imageVerify.UseCache)
			if err != nil {
				iv.logger.Error(err, "error occurred during cache get", "image", image)
			} else {
				isInCache = found
			}
		}

		var ruleResp *engineapi.RuleResponse
		var digest string
		if isInCache {
			iv.logger.V(2).Info("cache entry found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
			ruleResp = engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, "verified from cache", iv.rule.ReportProperties)
			digest = imageInfo.Digest
		} else {
			iv.logger.V(2).Info("cache entry not found", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
			ruleResp, digest = iv.verifyImage(ctx, imageVerify, imageInfo, cfg)
			if ruleResp != nil && ruleResp.Status() == engineapi.RuleStatusPass {
				if iv.ivCache != nil {
					setted, err := iv.ivCache.Set(ctx, iv.policyContext.Policy(), iv.rule.Name, image, imageVerify.UseCache)
					if err != nil {
						iv.logger.Error(err, "error occurred during cache set", "image", image)
					} else {
						if setted {
							iv.logger.V(4).Info("successfully set cache", "namespace", iv.policyContext.Policy().GetNamespace(), "policy", iv.policyContext.Policy().GetName(), "ruleName", iv.rule.Name, "imageRef", image)
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
				iv.ivm.Add(image, ruleStatusToImageVerificationStatus(ruleResp.Status()))
			}
			responses = append(responses, ruleResp)
		}
	}
	return patches, responses
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
		if deadline, ok := ctx.Deadline(); ok {
			// debug whether we have enough time to validate images for multi-containers pods
			iv.logger.V(4).Info("starting image verification", "path", path, "image", image, "deadlineRemaining", time.Until(deadline).String())
		} else {
			iv.logger.V(4).Info("verifying attestors", "path", path)
		}
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

// handle registry network errors as a rule error (instead of a policy failure)
func (iv *imageVerifier) handleRegistryErrors(image string, err error) *engineapi.RuleResponse {
	msg := fmt.Sprintf("failed to verify image %s: %s", image, err.Error())
	var netErr *net.OpError
	isNetErr := errors.As(err, &netErr)
	isContextCanceled := errors.Is(err, context.Canceled)
	isDeadlineExceeded := errors.Is(err, context.DeadlineExceeded)
	if isNetErr || isContextCanceled || isDeadlineExceeded {
		iv.logger.V(4).Info("image verification infrastructure error",
			"image", image,
			"networkError", isNetErr,
			"contextCanceled", isContextCanceled,
			"deadlineExceeded", isDeadlineExceeded,
		)
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
				return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), ""
			}
		}

		iv.logger.V(4).Info("attestation checks passed", "path", path, "image", imageInfo.String(), "type", attestation.Type)
	}

	if iv.rule.HasValidateImageVerification() {
		for _, imageVerify := range iv.rule.VerifyImages {
			if err := iv.validate(imageVerify, ctx); err != nil {
				msg := fmt.Sprintf("validation in verifyImages failed: %v", err)
				iv.logger.Error(err, "validation in verifyImages failed")
				return engineapi.RuleFail(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), imageInfo.Digest
			}
		}
		msg := fmt.Sprintf("verifyImages validation is passed in %v rule", iv.rule.Name)
		return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), imageInfo.Digest
	}

	msg := fmt.Sprintf("verified image attestations for %s", image)
	iv.logger.V(2).Info(msg)
	return engineapi.RulePass(iv.rule.Name, engineapi.ImageVerify, msg, iv.rule.ReportProperties), imageInfo.Digest
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
	statementsByPredicate, types := buildStatementMap(statements)
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
